package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	app := &App{
		ListenAddr:  ":8080",
		Router:      mux.NewRouter(),
		moduleStore: NewModuleStore(),
	}
	app.Router.
		HandleFunc("/.well-know/terraform.json", ServiceDiscovery).
		Methods("GET")
	app.Router.
		HandleFunc("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions()).
		Methods("GET")
	//app.Router.
	//	HandleFunc("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload()).
	//	Methods("GET")
	//app.Router.
	//	Use(app.TokenAuth)
	app.Router.NotFoundHandler = app.Router.NewRoute().HandlerFunc(http.NotFound).GetHandler()

	srv := http.Server{
		Addr:              app.ListenAddr,
		Handler:           handlers.LoggingHandler(os.Stdout, app.Router),
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Starting HTTP server, listening on %s", app.ListenAddr)
	srv.ListenAndServe()
}

type App struct {
	ListenAddr string
	Router     *mux.Router

	authTokens  []string
	moduleStore *ModuleStore
}

//func (app *App) LoadAuthTokenFile(filepath string) {
//	b, err := os.ReadFile(filepath)
//	if err != nil {
//		log.Panicf("LoadAuthTokenFile: %+v", err)
//	}
//
//	if err := json.Unmarshal(b, &app.authTokens); err != nil {
//		log.Panicf("LoadAuthTokenFile: %+v", err)
//	}
//}

// TokenAuth is a middleware function for token header authentication.
func (app *App) TokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		tokenType := auth[0]
		token := auth[1]

		if tokenType != "Bearer" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		for _, t := range app.authTokens {
			if t == token {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}

// ModuleStore stores metadata for available modules.
type ModuleStore struct {
	versions map[string][]string
}

func NewModuleStore() *ModuleStore {
	return &ModuleStore{
		versions: make(map[string][]string),
	}
}

// Set overwrites metadata for a module.
func (ms *ModuleStore) Set(module string, versions []string) {
	ms.versions[module] = versions
}

// Get returns the metadata for a module.
func (ms *ModuleStore) Get(module string) []string {
	return ms.versions[module]
}

// HasVersion tells whether a specific version is available for a specific module.
// Will return `false` if the module doesn't exist.
func (ms *ModuleStore) HasVersion(moduleName, version string) bool {
	module := ms.Get(moduleName)
	if module == nil {
		return false
	}
	for _, v := range module {
		if v == version {
			return true
		}
	}
	return false
}

// ServiceDiscovery is a handler that returns a JSON payload for Terraform service discovery.
//
// Given a hostname, discovery begins by forming an initial discovery URL using
// that hostname with the https: scheme and the fixed path /.well-known/terraform.json
// - https://www.terraform.io/internals/login-protocol
// - https://www.terraform.io/internals/module-registry-protocol
func ServiceDiscovery(w http.ResponseWriter, r *http.Request) {
	spec := []byte(`{
  "modules.v1": "/terraform/modules/v1/",
  "login.v1": {
    "client": "terraform-cli",
    "grant_types": ["authz_code"],
    "authz": "/oauth/authorization",
    "token": "/oauth/token",
    "ports": [10000, 10010]
  }
}`)

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(spec); err != nil {
		log.Printf("ServiceDiscovery: %+v", err)
	}
}

// ModuleVersions returns a handler that returns a list of available versions for a module.
// - https://www.terraform.io/internals/module-registry-protocol#list-available-versions-for-a-specific-module
func (app *App) ModuleVersions() http.HandlerFunc {
	// :namespace/:name/:system/versions
	urlPattern := regexp.MustCompile(`([\w-]+/[\w-]+/[\w-]+)/versions$`)

	return func(w http.ResponseWriter, r *http.Request) {
		m := urlPattern.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		module := m[1]
		versions := app.moduleStore.Get(module)
		if versions == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(versions)
		if err != nil {
			log.Printf("ModuleVersions: %+v", err)
		}

		if _, err := w.Write(b); err != nil {
			log.Printf("ModuleVersions: %+v", err)
		}
	}
}

// DownloadModule returns a handler that returns a download link for a specific version of a module.
// https://www.terraform.io/internals/module-registry-protocol#download-source-code-for-a-specific-module-version
func (app *App) DownloadModule() http.HandlerFunc {
	urlPat := regexp.MustCompile(`(?P<namespace>[\w-]+)/(?P<name>[\w-]+)/(?P<provider>[\w-]+)/(?P<version>[\w-.]+)/download$`)

	return func(w http.ResponseWriter, r *http.Request) {
		m := urlPat.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		var (
			namespace   = m[urlPat.SubexpIndex("namespace")]
			name        = m[urlPat.SubexpIndex("name")]
			provider    = m[urlPat.SubexpIndex("provider")]
			version     = m[urlPat.SubexpIndex("version")]
			module      = fmt.Sprintf("%s/%s/%s", namespace, name, provider)
			downloadURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", namespace, name, version)
		)

		if !app.moduleStore.HasVersion(module, version) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		w.Header().Set("X-Terraform-Get", downloadURL)
		w.WriteHeader(http.StatusNoContent)
	}
}
