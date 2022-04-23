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
		Handler:           app.router,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Starting HTTP server, listening on %s", app.ListenAddr)
	srv.ListenAndServeTLS("/home/n645863/tmp/ssl-selfsigned/cert.crt", "/home/n645863/tmp/ssl-selfsigned/cert.key")
}

type App struct {
	ListenAddr     string
	IsAuthDisabled bool

	router      *chi.Mux
	authTokens  []string
	moduleStore *ModuleStore
	ghclient    *GitHubClient
}

func (app *App) SetupRouter() {
	app.router = chi.NewRouter()
	app.router.Use(middleware.Logger)
	app.router.Use(app.TokenAuth)
	app.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Terraform Registry\n")
	})
	app.router.Get("/.well-known/terraform.json", app.ServiceDiscovery())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload())

	// Work-around to trigger log handler on non-matching 404's
	//app.router.NotFoundHandler = app.router.NewRoute().HandlerFunc(http.NotFound).GetHandler()
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

// ServiceDiscovery is a handler that returns a JSON payload for Terraform service discovery.
//
// Given a hostname, discovery begins by forming an initial discovery URL using
// that hostname with the https: scheme and the fixed path /.well-known/terraform.json
// - https://www.terraform.io/internals/login-protocol
// - https://www.terraform.io/internals/module-registry-protocol
func (app *App) ServiceDiscovery() http.HandlerFunc {
	spec := struct {
		ModulesV1 string `json:"modules.v1"`
		LoginV1   struct {
			Client     string   `json:"client"`
			GrantTypes []string `json:"grant_types"`
			Authz      string   `json:"authz"`
			Token      string   `json:"token"`
			Ports      []int    `json:"ports"`
		} `json:"login.v1"`
	}{}

	spec.ModulesV1 = "/v1/modules/"
	spec.LoginV1.Client = "terraform-cli"
	spec.LoginV1.GrantTypes = []string{"authz_code"}
	spec.LoginV1.Authz = "/oauth/authorization"
	spec.LoginV1.Token = "/oauth/token"
	spec.LoginV1.Ports = []int{10000, 10010}

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
// - https://www.terraform.io/internals/module-registry-protocol#list-available-versions-for-a-specific-module
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
