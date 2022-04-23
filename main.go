package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	log.Default().SetFlags(log.Lshortfile)

	app := &App{
		ListenAddr:  ":8080",
		moduleStore: NewModuleStore(),
		ghclient:    NewGitHubClient(os.Getenv("GITHUB_TOKEN")),
	}
	app.SetupRouter()

	if err := app.ghclient.TestCredentials(context.Background()); err != nil {
		log.Fatalf("error: github credential test: %v", err)
	}

	//moduleRepos, err := app.ghclient.ListUserRepositoriesByTopic(context.Background(), "nrkno")
	//if err != nil {
	//	log.Fatalf("error: failed to get repositories: %v", err)
	//}

	//app.LoadGitHubRepositories(context.Background(), []string{
	//	"stigok/plattform-terraform-repository-release-test",
	//})

	srv := http.Server{
		Addr:              app.ListenAddr,
		Handler:           handlers.LoggingHandler(os.Stdout, app.router),
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Starting HTTP server, listening on %s", app.ListenAddr)
	srv.ListenAndServeTLS("/home/n645863/tmp/ssl-selfsigned/cert.crt", "/home/n645863/tmp/ssl-selfsigned/cert.key")
}

func (app *App) SetupRouter() {
	app.router = mux.NewRouter()

	app.router.
		Use(app.TokenAuth)

	app.router.
		HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Terraform Registry\n")
		})

	app.router.
		HandleFunc("/.well-known/terraform.json", ServiceDiscovery).
		Methods("GET")

	app.router.
		HandleFunc("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions()).
		Methods("GET")

	app.router.
		HandleFunc("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload()).
		Methods("GET")

	// Work-around to trigger log handler on non-matching 404's
	app.router.NotFoundHandler = app.router.NewRoute().HandlerFunc(http.NotFound).GetHandler()
}

type Module struct {
	Namespace string
	Name      string
	System    string
	// Versions is a map where the key is a version string and value s the git ref
	Versions map[string]string
}

func (m *Module) String() string {
	return fmt.Sprintf("%s/%s/%s", m.Namespace, m.Name, m.System)
}

func (m *Module) HasVersion(version string) bool {
	for _, v := range m.Versions {
		if v == version {
			return true
		}
	}
	return false
}

type ModuleStore struct {
	store map[string]Module
	mut   sync.RWMutex
}

func NewModuleStore() *ModuleStore {
	return &ModuleStore{
		store: make(map[string]Module),
	}
}

func (ms *ModuleStore) Get(key string) *Module {
	ms.mut.RLock()
	defer ms.mut.RUnlock()

	m, ok := ms.store[key]
	if !ok {
		return nil
	}
	return &m
}

func (ms *ModuleStore) Set(key string, m Module) {
	ms.mut.Lock()
	defer ms.mut.Unlock()
	ms.store[key] = m
}

type App struct {
	ListenAddr string
	router     *mux.Router

	authTokens  []string
	moduleStore *ModuleStore
	ghclient    *GitHubClient
}

func (app *App) LoadGitHubRepositories(ctx context.Context, repos []string) {
	for _, repoRef := range repos {
		parts := strings.SplitN(repoRef, "/", 2)
		if len(parts) != 2 {
			log.Printf("error: LoadGitHubRepositories: invalid repo reference '%s'. should be in the form 'owner/repo'.", repos)
			continue
		}

		var (
			repoOwner = parts[0]
			repoName  = parts[1]
		)

		tags, err := app.ghclient.GetRepoTags(ctx, repoOwner, repoName)
		if err != nil {
			log.Printf("error: LoadGitHubRepositories: %v", err)
			continue
		}

		if len(tags) == 0 {
			log.Printf("debug: LoadGitHubRepositories: no tags for repo %s/%s found.", repoOwner, repoName)
			continue
		}

		m := Module{
			Namespace: repoOwner,
			Name:      repoName,
			System:    "generic", // Required by Terraform, but we don't want to segment the modules into systems (could be any string).
			Versions:  make(map[string]string, len(tags)),
		}
		for _, tag := range tags {
			m.Versions[*tag.Name] = strings.TrimPrefix(*tag.Name, "v")
		}

		app.moduleStore.Set(m.String(), m)
	}
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
			log.Println("error: TokenAuth: invalid or missing Authorization header.")
			return
		}

		tokenType := auth[0]
		token := auth[1]

		if tokenType != "Bearer" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			log.Printf("error: TokenAuth: Authorization header value not of type 'Bearer', but '%s'.", tokenType)
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
  "modules.v1": "/v1/modules/",
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
		log.Printf("error: ServiceDiscovery: %+v", err)
	}
}

type ModuleVersionsResponse struct {
	Modules []ModuleVersionsResponseModule `json:"modules"`
}

type ModuleVersionsResponseModule struct {
	Versions []ModuleVersionsResponseModuleVersion `json:"versions"`
}

type ModuleVersionsResponseModuleVersion struct {
	Version string `json:"version"`
}

// ModuleVersions returns a handler that returns a list of available versions for a module.
// - https://www.terraform.io/internals/module-registry-protocol#list-available-versions-for-a-specific-module
func (app *App) ModuleVersions() http.HandlerFunc {
	urlPat := regexp.MustCompile(`(?P<namespace>[\w-]+)/(?P<name>[\w-]+)/(?P<provider>[\w-]+)/versions$`)

	return func(w http.ResponseWriter, r *http.Request) {
		m := urlPat.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		var (
			namespace = m[urlPat.SubexpIndex("namespace")]
			name      = m[urlPat.SubexpIndex("name")]
			provider  = m[urlPat.SubexpIndex("provider")]
			key       = fmt.Sprintf("%s/%s/%s", namespace, name, provider)
		)

		module := app.moduleStore.Get(key)
		if module == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleVersions: module '%s' not found.", key)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		respObj := ModuleVersionsResponse{
			Modules: make([]ModuleVersionsResponseModule, 1),
		}
		for v, _ := range module.Versions {
			respObj.Modules[0].Versions = append(respObj.Modules[0].Versions, ModuleVersionsResponseModuleVersion{Version: v})
		}

		b, err := json.Marshal(respObj)
		if err != nil {
			log.Printf("error: ModuleVersions: %+v", err)
		}

		//_, err = fmt.Fprintf(w, `{"modules":[{"versions":%s}]}`, b)
		if _, err := w.Write(b); err != nil {
			log.Printf("error: ModuleVersions: %+v", err)
		}
	}
}

// ModuleDownload returns a handler that returns a download link for a specific version of a module.
// https://www.terraform.io/internals/module-registry-protocol#download-source-code-for-a-specific-module-version
func (app *App) ModuleDownload() http.HandlerFunc {
	urlPat := regexp.MustCompile(`(?P<namespace>[\w-]+)/(?P<name>[\w-]+)/(?P<provider>[\w-]+)/(?P<version>[\w-.]+)/download$`)

	return func(w http.ResponseWriter, r *http.Request) {
		m := urlPat.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		key := fmt.Sprintf("%s/%s/%s", m[urlPat.SubexpIndex("namespace")], m[urlPat.SubexpIndex("name")], m[urlPat.SubexpIndex("provider")])
		module := app.moduleStore.Get(key)
		if module == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleDownload: module '%s' not found.", module)
			return
		}

		version := m[urlPat.SubexpIndex("version")]
		tag, ok := module.Versions[version]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleDownload: version '%s' not found for module '%s'.", version, module)
			return
		}

		//w.Header().Set("X-Terraform-Get", version.DownloadURL)
		w.Header().Set(
			"X-Terraform-Get",
			fmt.Sprintf("git::ssh://git@github.com/%s/%s.git?ref=%s", module.Namespace, module.Name, tag),
		)
		w.WriteHeader(http.StatusNoContent)
	}
}
