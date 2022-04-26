package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nrkno/terraform-registry/pkg/store"
)

var (
	WelcomeMessage = []byte("Terraform Registry\nhttps://github.com/nrkno/terraform-registry\n")
)

type App struct {
	// Registry server HTTP listen address
	ListenAddr string `split_words:"true" default:":8080" required:"true"`
	// Whether to disable auth
	IsAuthDisabled bool `envconfig:"AUTH_DISABLED"`
	// File containing newline separated strings with valid auth tokens
	AuthTokenFile string `split_words:"true"`
	// API access token for the GitHub API
	GitHubToken string `split_words:"true" required:"true"`
	// The GitHub org name to use for module discovery
	GitHubOrgName string `split_words:"true" required:"true"`
	// The GitHub repository topic to match. Will only expose repositories whose topics contain this.
	GitHubRepoMatchTopic string `split_words:"true" default:"terraform-module" required:"true"`
	// Whether to enable TLS termination. This requires TLSCertFile and TLSKeyFile.
	TLSEnabled  bool   `split_words:"true"`
	TLSCertFile string `split_words:"true"`
	TLSKeyFile  string `split_words:"true"`

	router      *chi.Mux
	authTokens  []string
	moduleStore store.ModuleStore
}

func (app *App) SetModuleStore(s store.ModuleStore) {
	app.moduleStore = s
}

func (app *App) GetAuthTokens() []string {
	return app.authTokens
}

func (app *App) SetAuthTokens(authTokens []string) {
	app.authTokens = authTokens
}

func (app *App) LoadAuthTokens() error {
	if app.AuthTokenFile == "" {
		return fmt.Errorf("LoadModules: AuthTokenFile is not specified")
	}

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

func (app *App) SetupRoutes() {
	app.router = chi.NewRouter()
	app.router.Use(middleware.Logger)
	app.router.Use(app.TokenAuth)
	app.router.Get("/", app.Index())
	app.router.Get("/health", app.Health())
	app.router.Get("/.well-known/terraform.json", app.ServiceDiscovery())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload())
}

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
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
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(WelcomeMessage); err != nil {
			log.Printf("error: Index: %v", err)
		}
	}
}

func (app *App) Health() http.HandlerFunc {
	resp := []byte("OK")
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(resp); err != nil {
			log.Printf("error: Health: %v", err)
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
		)

		versions, err := app.moduleStore.ListModuleVersions(r.Context(), namespace, name, system)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleVersions: %v", err)
			return
		}

		respObj := ModuleVersionsResponse{
			Modules: []ModuleVersionsResponseModule{
				ModuleVersionsResponseModule{},
			},
		}
		for _, v := range versions {
			respObj.Modules[0].Versions = append(respObj.Modules[0].Versions, ModuleVersionsResponseModuleVersion{Version: v.Version})
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
		)

		ver, err := app.moduleStore.GetModuleVersion(r.Context(), namespace, name, system, version)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			log.Printf("error: ModuleDownload: %v", err)
			return
		}

		w.Header().Set("X-Terraform-Get", ver.SourceURL)
		w.WriteHeader(http.StatusNoContent)
	}
}
