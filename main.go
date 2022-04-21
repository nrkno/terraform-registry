package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	app := &App{
		ListenAddr: ":8080",
		Router:     mux.NewRouter(),
	}
	//app.Router.
	//	Use(app.TokenAuth)
	app.Router.
		HandleFunc("/.well-know/terraform.json", ServiceDiscovery).
		Methods("GET")
	app.Router.
		HandleFunc("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions()).
		Methods("GET")
	//app.Router.
	//	HandleFunc("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload()).
	//	Methods("GET")
	app.ListenAndServe()
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

func (app *App) ListenAndServe() error {
	srv := http.Server{
		Addr:              app.ListenAddr,
		Handler:           app.Router,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}

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

type ModuleStore struct {
	versions map[string][]string
}

func NewModuleStore() *ModuleStore {
	return &ModuleStore{
		versions: make(map[string][]string),
	}
}

func (ms *ModuleStore) Set(module string, versions []string) {
	ms.versions[module] = versions
}

func (ms *ModuleStore) Get(module string) []string {
	return ms.versions[module]
}
