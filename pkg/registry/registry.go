// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package registry

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nrkno/terraform-registry/pkg/core"
)

var (
	// WelcomeMessage is the message returned from the index route.
	WelcomeMessage = []byte("Terraform Registry\nhttps://github.com/nrkno/terraform-registry\n")
)

type Registry struct {
	// Whether to disable auth
	IsAuthDisabled bool
	// File containing newline separated strings with valid auth tokens
	AuthTokenFile string

	router      *chi.Mux
	authTokens  []string
	moduleStore core.ModuleStore
}

func NewRegistry() *Registry {
	reg := &Registry{
		IsAuthDisabled: false,
		AuthTokenFile:  "",
	}
	reg.setupRoutes()
	return reg
}

// SetModuleStore sets the active module store for this instance.
func (app *Registry) SetModuleStore(s core.ModuleStore) {
	app.moduleStore = s
}

// GetAuthTokens gets the valid auth tokens configured for this instance.
func (app *Registry) GetAuthTokens() []string {
	return app.authTokens
}

// SetAuthTokens sets the valid auth tokens configured for this instance.
func (app *Registry) SetAuthTokens(authTokens []string) {
	app.authTokens = authTokens
}

// Initialises and configures the HTTP router. Must be called before starting the server (`ServeHTTP`).
func (app *Registry) setupRoutes() {
	app.router = chi.NewRouter()
	app.router.Use(middleware.Logger)
	app.router.Use(app.TokenAuth)
	app.router.Get("/", app.Index())
	app.router.Get("/health", app.Health())
	app.router.Get("/.well-known/terraform.json", app.ServiceDiscovery())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/versions", app.ModuleVersions())
	app.router.Get("/v1/modules/{namespace}/{name}/{system}/{version}/download", app.ModuleDownload())
}

func (app *Registry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.router.ServeHTTP(w, r)
}

// TokenAuth is a middleware function for token header authentication.
func (app *Registry) TokenAuth(next http.Handler) http.Handler {
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

func (app *Registry) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(WelcomeMessage); err != nil {
			log.Printf("error: Index: %v", err)
		}
	}
}

func (app *Registry) Health() http.HandlerFunc {
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
func (app *Registry) ServiceDiscovery() http.HandlerFunc {
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
func (app *Registry) ModuleVersions() http.HandlerFunc {
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
func (app *Registry) ModuleDownload() http.HandlerFunc {
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
