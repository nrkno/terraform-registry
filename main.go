package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	app := &App{
		ListenAddr: ":8080",
		Router:     mux.NewRouter(),
	}
	app.Router.Use(app.TokenAuth)
	app.Router.HandleFunc("/.well-know/terraform.json", ServiceDiscovery)
	app.ListenAndServe()
}

type App struct {
	ListenAddr string
	Router     *mux.Router

	authTokens []string
}

func (app *App) LoadAuthTokenFile(filepath string) {
	b, err := os.ReadFile(filepath)
	if err != nil {
		log.Panicf("LoadAuthTokenFile: %+v", err)
	}

	if err := json.Unmarshal(b, &app.authTokens); err != nil {
		log.Panicf("LoadAuthTokenFile: %+v", err)
	}
}

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

// Middleware for token header authentication.
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

// https://www.terraform.io/internals/module-registry-protocol#module-addresses
type Module struct {
	// The hostname of the module registry that serves this module
	Hostname string
	// The name of a namespace, unique on a particular hostname,
	// that can contain one or more modules that are somehow related.
	Namespace string
	// The module name, which generally names the abstraction that
	// the module is intending to create.
	Name string
	// The name of a remote system that the module is primarily written to target.
	// The system name commonly matches the type portion of the address of an official
	// provider, like aws or azurerm in the above examples, but that is not required
	// and so you can use whichever system keywords make sense for the organization
	// of your particular registry.
	System string
}
