package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kelseyhightower/envconfig"
)

type App struct {
	// Registry server HTTP listen address
	ListenAddr string `split_words:"true" default:":8080"`
	// Whether to disable auth
	IsAuthDisabled bool `envconfig:"AUTH_DISABLED"`
	// File containing newline separated strings with valid auth tokens
	AuthTokenFile string `split_words:"true"`
	// API access token for the GitHub API
	GitHubToken string `split_words:"true"`

	router      *chi.Mux
	authTokens  []string
	moduleStore *ModuleStore
	ghclient    *GitHubClient
}

func main() {
	log.Default().SetFlags(log.Lshortfile)

	app := new(App)
	envconfig.MustProcess("", &app)

	app.moduleStore = NewModuleStore()
	app.ghclient = NewGitHubClient(app.GitHubToken)

	app.SetupRoutes()

	if err := app.ghclient.TestCredentials(context.Background()); err != nil {
		log.Fatalf("error: github credential test: %v", err)
	}

	if !app.IsAuthDisabled {
		if err := app.LoadAuthTokens(); err != nil {
			log.Fatalf("error: failed to load auth tokens: %v", err)
		}
		log.Println("info: authentication is enabled")
		log.Printf("info: loaded %d auth tokens", len(app.authTokens))
	} else {
		log.Println("warning: authentication is disabled")
	}

	//moduleRepos, err := app.ghclient.ListUserRepositoriesByTopic(context.Background(), "nrkno", "terraform-module")
	//if err != nil {
	//	log.Fatalf("error: failed to get repositories: %v", err)
	//}

	//app.LoadGitHubRepositories(context.Background(), []string{
	//	"stigok/plattform-terraform-repository-release-test",
	//})

	srv := http.Server{
		Addr:              app.ListenAddr,
		Handler:           app.router,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Starting HTTP server, listening on %s", app.ListenAddr)
	srv.ListenAndServeTLS("/home/n645863/tmp/ssl-selfsigned/cert.crt", "/home/n645863/tmp/ssl-selfsigned/cert.key")
}

func (app *App) LoadAuthTokens() error {
	b, err := os.ReadFile(app.AuthTokenFile)
	if err != nil {
		return fmt.Errorf("LoadModules: %w", err)
	}

	tokens := strings.Split(string(b), "\n")
	for _, token := range tokens {
		if token = strings.TrimSpace(token); token != "" {
			app.authTokens = append(app.authTokens, token)
		}
	}
	return nil
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

		tags, err := app.ghclient.ListAllRepoTags(ctx, repoOwner, repoName)
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

func (app *App) SetupRoutes() {
	app.router = chi.NewRouter()
	app.router.Use(middleware.Logger)
	app.router.Use(app.TokenAuth)
	app.router.Get("/", app.Index())
	app.router.Get("/.well-known/terraform.json", app.ServiceDiscovery())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload())
}

// TokenAuth is a middleware function for token header authentication.
func (app *App) TokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.IsAuthDisabled {
			next.ServeHTTP(w, r)
			return
		}

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

func (app *App) Index() http.HandlerFunc {
	resp := []byte("Terraform Registry\n")
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(resp); err != nil {
			log.Printf("error: Index: %v", err)
		}
	}
}

type ServiceDiscoveryResponse struct {
	ModulesV1 string                          `json:"modules.v1"`
	LoginV1   ServiceDiscoveryResponseLoginV1 `json:"login.v1"`
}

type ServiceDiscoveryResponseLoginV1 struct {
	Client     string   `json:"client"`
	GrantTypes []string `json:"grant_types"`
	Authz      string   `json:"authz"`
	Token      string   `json:"token"`
	Ports      []int    `json:"ports"`
}

// ServiceDiscovery returns a handler that returns a JSON payload for Terraform service discovery.
// https://www.terraform.io/internals/login-protocol
// https://www.terraform.io/internals/module-registry-protocol
func (app *App) ServiceDiscovery() http.HandlerFunc {
	spec := ServiceDiscoveryResponse{
		ModulesV1: "/v1/modules/",
		LoginV1: ServiceDiscoveryResponseLoginV1{
			Client:     "terraform-cli",
			GrantTypes: []string{"authz_code"},
			Authz:      "/oauth/authorization",
			Token:      "/oauth/token",
			Ports:      []int{10000, 10010},
		},
	}

	resp, err := json.Marshal(spec)
	if err != nil {
		log.Fatalf("error: ServiceDiscovery: %v", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Printf("error: ServiceDiscovery: %+v", err)
		}
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
// https://www.terraform.io/internals/module-registry-protocol#list-available-versions-for-a-specific-module
func (app *App) ModuleVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
			system    = chi.URLParam(r, "system")
			moduleKey = fmt.Sprintf("%s/%s/%s", namespace, name, system)
		)

		module := app.moduleStore.Get(moduleKey)
		if module == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleVersions: module '%s' not found.", moduleKey)
			return
		}

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

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(b); err != nil {
			log.Printf("error: ModuleVersions: %+v", err)
		}
	}
}

// ModuleDownload returns a handler that returns a download link for a specific version of a module.
// https://www.terraform.io/internals/module-registry-protocol#download-source-code-for-a-specific-module-version
func (app *App) ModuleDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
			system    = chi.URLParam(r, "system")
			version   = chi.URLParam(r, "version")
			moduleKey = fmt.Sprintf("%s/%s/%s", namespace, name, system)
		)

		module := app.moduleStore.Get(moduleKey)
		if module == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleDownload: module '%s' not found.", moduleKey)
			return
		}

		gitRef, ok := module.Versions[version]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleDownload: version '%s' not found for module '%s'.", version, module)
			return
		}

		w.Header().Set(
			"X-Terraform-Get",
			fmt.Sprintf("git::ssh://git@github.com/%s/%s.git?ref=%s", module.Namespace, module.Name, gitRef),
		)
		w.WriteHeader(http.StatusNoContent)
	}
}
