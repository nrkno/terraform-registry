// SPDX-FileCopyrightText: 2022 - 2025 NRK
//
// SPDX-License-Identifier: MIT

package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

var (
	// WelcomeMessage is the message returned from the index route.
	WelcomeMessage = []byte("Terraform Registry\nhttps://github.com/nrkno/terraform-registry\n")
)

// Registry implements the Terraform HTTP registry protocol.
// Should not be instantiated directly. Use `NewRegistry` instead.
type Registry struct {
	// Whether to disable auth
	IsAuthDisabled bool
	// Whether to disable HTTP access log
	IsAccessLogDisabled bool
	// Paths to ignore request logging for
	AccessLogIgnoredPaths []string

	// Whether to enable provider registry support
	IsProviderEnabled bool

	// Secret used to issue JTW for protecting the /download/provider/ route
	AssetDownloadAuthSecret []byte

	router        *chi.Mux
	authTokens    map[string]string
	moduleStore   core.ModuleStore
	providerStore core.ProviderStore
	tokenMut      sync.RWMutex

	logger *zap.Logger
}

func NewRegistry(logger *zap.Logger) *Registry {
	if logger == nil {
		logger = zap.NewNop()
	}

	reg := &Registry{
		IsAuthDisabled:    false,
		IsProviderEnabled: false,
		logger:            logger,
	}
	reg.setupRoutes()
	return reg
}

// SetModuleStore sets the active module store for this instance.
func (reg *Registry) SetModuleStore(s core.ModuleStore) {
	reg.moduleStore = s
}

// SetProviderStore sets the active provider store for this instance.
func (reg *Registry) SetProviderStore(s core.ProviderStore) {
	reg.providerStore = s
}

// GetAuthTokens gets the valid auth tokens configured for this instance.
func (reg *Registry) GetAuthTokens() map[string]string {
	reg.tokenMut.RLock()
	defer reg.tokenMut.RUnlock()

	// Make sure map can't be modified indirectly
	m := make(map[string]string, len(reg.authTokens))
	for k, v := range reg.authTokens {
		m[k] = v
	}
	return m
}

// SetAuthTokens sets the valid auth tokens configured for this instance.
func (reg *Registry) SetAuthTokens(authTokens map[string]string) {
	// Make sure map can't be modified indirectly
	m := make(map[string]string, len(authTokens))
	for k, v := range authTokens {
		m[k] = v
	}

	reg.tokenMut.Lock()
	reg.authTokens = m
	reg.tokenMut.Unlock()
}

// setupRoutes initialises and configures the HTTP router. Must be called before starting the server (`ServeHTTP`).
func (reg *Registry) setupRoutes() {
	reg.router = chi.NewRouter()
	reg.router.Use(reg.RequestLogger())
	reg.router.NotFound(reg.NotFound())
	reg.router.MethodNotAllowed(reg.MethodNotAllowed())
	reg.router.Get("/", reg.Index())
	reg.router.Get("/health", reg.Health())
	reg.router.Get("/.well-known/{name}", reg.ServiceDiscovery())

	// Only API routes are protected with authentication
	reg.router.Route("/v1", func(r chi.Router) {
		r.Use(reg.TokenAuth)
		r.Get("/modules/{namespace}/{name}/{provider}/versions", reg.ModuleVersions())
		r.Get("/modules/{namespace}/{name}/{provider}/{version}/download", reg.ModuleDownload())
		r.Get("/providers/{namespace}/{name}/versions", reg.ProviderVersions())
		r.Get("/providers/{namespace}/{name}/{version}/download/{os}/{arch}", reg.ProviderDownload())
	})

	reg.router.Route("/download/provider", func(r chi.Router) {
		r.Use(reg.ProviderDownloadAuth)
		r.Get("/{namespace}/{name}/{version}/asset/{assetName}", reg.ProviderAssetDownload())
	})
}

// SPDX-SnippetBegin
// SPDX-License-Identifier: MIT
// SPDX-SnippetCopyrightText: Copyright (c) 2021  Manfred Touron <oss@moul.io> (manfred.life)
// SDPX—SnippetName: Function to configure Zap logger with Chi HTTP router
// SPDX-SnippetComment: Original work at https://github.com/moul/chizap/blob/0ebf11a6a5535e3c6bb26f1236b2833ae7825675/chizap.go. All further changes are licensed under this file's main license.

// Request logger for Chi using Zap as the logger.
func (reg *Registry) RequestLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if reg.IsAccessLogDisabled {
				next.ServeHTTP(w, r)
				return
			}

			if slices.Contains(reg.AccessLogIgnoredPaths, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			wr := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				ua := wr.Header().Get("User-Agent")
				if ua == "" {
					ua = r.Header.Get("User-Agent")
				}

				reqLogger := reg.logger.With(
					zap.String("proto", r.Proto),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("size", wr.BytesWritten()),
					zap.Int("status", wr.Status()),
					zap.String("reqId", middleware.GetReqID(r.Context())),
					zap.Duration("responseTimeNSec", time.Since(t1)),
					zap.String("userAgent", ua),
				)

				reqLogger.Info("HTTP request")
			}()
			next.ServeHTTP(wr, r)
		})
	}
}

// SPDX-SnippetEnd

func (reg *Registry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reg.router.ServeHTTP(w, r)
}

// TokenAuth is a middleware function for token header authentication.
func (reg *Registry) TokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reg.IsAuthDisabled {
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			reg.logger.Debug("TokenAuth: Authorization header missing or empty")
			return
		}

		auth := strings.SplitN(header, " ", 2)
		if len(auth) != 2 {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			reg.logger.Debug("TokenAuth: Authorization header present, but invalid")
			return
		}

		tokenType := auth[0]
		token := auth[1]

		if tokenType != "Bearer" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			reg.logger.Debug("TokenAuth: unexpected authorization header value prefix",
				zap.String("actual", tokenType),
				zap.String("expected", "Bearer"),
			)
			return
		}

		for _, t := range reg.GetAuthTokens() {
			if t == token {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}

func (reg *Registry) NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func (reg *Registry) MethodNotAllowed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (reg *Registry) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if _, err := w.Write(WelcomeMessage); err != nil {
			reg.logger.Error("Index", zap.Error(err))
		}
	}
}

type HealthResponse struct {
	Status string `json:"status"`
}

// Health is the endpoint to be checked to know the runtime health of the registry.
// In its current implementation it will always report as healthy, i.e. it only
// reports that the HTTP server still handles requests.
func (reg *Registry) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status: "OK",
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			reg.logger.Error("Health", zap.Error(err))
		}
	}
}

type ServiceDiscoveryResponse struct {
	ModulesV1   string `json:"modules.v1"`
	ProvidersV1 string `json:"providers.v1"`
}

// ServiceDiscovery returns a handler that returns a JSON payload for Terraform service discovery.
// https://www.terraform.io/internals/module-registry-protocol
func (reg *Registry) ServiceDiscovery() http.HandlerFunc {
	spec := ServiceDiscoveryResponse{
		ModulesV1:   "/v1/modules/",
		ProvidersV1: "/v1/providers/",
	}

	resp, err := json.Marshal(spec)
	if err != nil {
		reg.logger.Panic("ServiceDiscovery", zap.Error(err))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if chi.URLParam(r, "name") != "terraform.json" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			reg.logger.Error("ServiceDiscovery", zap.Error(err))
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
func (reg *Registry) ModuleVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
			provider  = chi.URLParam(r, "provider")
		)

		versions, err := reg.moduleStore.ListModuleVersions(r.Context(), namespace, name, provider)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Debug("ListModuleVersions", zap.Error(err))
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
			reg.logger.Error("ModuleVersions", zap.Error(err))
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(b); err != nil {
			reg.logger.Error("ModuleVersions", zap.Error(err))
		}
	}
}

// ModuleDownload returns a handler that returns a download link for a specific version of a module.
// https://www.terraform.io/internals/module-registry-protocol#download-source-code-for-a-specific-module-version
func (reg *Registry) ModuleDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
			provider  = chi.URLParam(r, "provider")
			version   = chi.URLParam(r, "version")
		)

		ver, err := reg.moduleStore.GetModuleVersion(r.Context(), namespace, name, provider, version)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("GetModuleVersion", zap.Error(err))
			return
		}

		w.Header().Set("X-Terraform-Get", ver.SourceURL)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ProviderVersions returns a handler that returns a list of available versions for a provider.
// https://developer.hashicorp.com/terraform/internals/provider-registry-protocol#list-available-versions
func (reg *Registry) ProviderVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
		)

		ver, err := reg.providerStore.ListProviderVersions(r.Context(), namespace, name)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("ListProviderVersions", zap.Error(err))
			return
		}

		err = json.NewEncoder(w).Encode(ver)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("ListProviderVersions", zap.Error(err))
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// ProviderDownload returns a handler that returns a download link for a specific version of a provider.
// https://developer.hashicorp.com/terraform/internals/provider-registry-protocol#find-a-provider-package
func (reg *Registry) ProviderDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			namespace = chi.URLParam(r, "namespace")
			name      = chi.URLParam(r, "name")
			version   = chi.URLParam(r, "version")
			os        = chi.URLParam(r, "os")
			arch      = chi.URLParam(r, "arch")
		)

		provider, err := reg.providerStore.GetProviderVersion(r.Context(), namespace, name, version, os, arch)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("GetProviderVersion", zap.Error(err))
			return
		}

		// Terraform does not send registry auth headers when downloading assets, we add a
		// token as query parameter to be able to protect the download routes which are
		// used when assets are not publicly available.
		if strings.HasPrefix(provider.DownloadURL, "/download") && !reg.IsAuthDisabled {
			// Create a copy of the provider before we modify URLs
			provider = provider.Copy()

			// create a token valid for 10 seconds. Should be more than enough.
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Second * 10)),
				Issuer:    "terraform-registry",
			})

			tokenString, err := token.SignedString(reg.AssetDownloadAuthSecret)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				reg.logger.Error("GetProviderVersion: unable to create token", zap.Error(err))
				return
			}

			provider.DownloadURL = fmt.Sprintf("%s?token=%s", provider.DownloadURL, tokenString)
			provider.SHASumsURL = fmt.Sprintf("%s?token=%s", provider.SHASumsURL, tokenString)
			provider.SHASumsSignatureURL = fmt.Sprintf("%s?token=%s", provider.SHASumsSignatureURL, tokenString)
		}

		err = json.NewEncoder(w).Encode(provider)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("GetProviderVersion", zap.Error(err))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// ProviderAssetDownload returns a handler that returns a provider asset.
// When a provider binaries is not hosted on a public webserver, this handler can be used to fetch
// assets directly from the store
func (reg *Registry) ProviderAssetDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			owner     = chi.URLParam(r, "namespace")
			repo      = chi.URLParam(r, "name")
			tag       = chi.URLParam(r, "version")
			assetName = chi.URLParam(r, "assetName")
		)

		asset, err := reg.providerStore.GetProviderAsset(r.Context(), owner, repo, tag, assetName)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			reg.logger.Error("ProviderAssetDownload", zap.Error(err))
			return
		}
		defer asset.Close()

		written, err := io.Copy(w, asset)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			reg.logger.Error("ProviderAssetDownload", zap.Error(err))
			return
		}

		reg.logger.Debug(fmt.Sprintf("ProviderAssetDownload: wrote %d bytes to response", written))
		w.WriteHeader(http.StatusOK)
	}
}

func (reg *Registry) ProviderDownloadAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reg.IsAuthDisabled {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := r.URL.Query().Get("token")
		if tokenString == "" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			reg.logger.Debug("ProviderDownloadAuth: Token query parameter missing or empty")
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return reg.AssetDownloadAuthSecret, nil
		})

		switch {
		case token.Valid:
			next.ServeHTTP(w, r)
			return
		case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
			reg.logger.Error("ProviderDownloadAuth: Token is expired or not valid yet")
		default:
			reg.logger.Error("ProviderDownloadAuth: Token not valid")
		}

		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}
