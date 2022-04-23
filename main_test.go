package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/matryer/is"
)

func TestServiceDiscovery(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)

	w := httptest.NewRecorder()
	ServiceDiscovery(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	is.Equal(resp.StatusCode, 200)
	is.Equal(resp.Header.Get("Content-Type"), "application/json")
	is.True(len(body) > 1)

	err := json.Unmarshal(body, &struct{}{})
	is.NoErr(err)
}

func TestTokenAuth(t *testing.T) {
	is := is.New(t)

	app := App{
		router: mux.NewRouter(),
		authTokens: []string{
			"valid",
		},
	}
	app.router.Use(app.TokenAuth)
	app.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "authenticated")
	})

	testcases := []struct {
		name   string
		token  string
		status int
		body   string
	}{
		{
			"empty token",
			"",
			http.StatusForbidden,
			"Forbidden\n",
		},
		{
			"invalid token",
			"foobar",
			http.StatusForbidden,
			"Forbidden\n",
		},
		{
			"valid token",
			"valid",
			http.StatusOK,
			"authenticated",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()

			app.router.ServeHTTP(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)

			is.Equal(resp.StatusCode, tc.status)
			is.Equal(string(body), tc.body)
		})
	}
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)

	app := App{
		moduleStore: NewModuleStore(),
	}

	app.moduleStore.Set("hashicorp/consul/aws", Module{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions: []string{
			"1.1.1",
			"2.2.2",
			"3.3.3",
		},
	})

	testcases := []struct {
		name   string
		module string
		status int
		body   string
	}{
		{
			"valid module",
			"hashicorp/consul/aws",
			http.StatusOK,
			`{"modules":[{"versions":[{"version":"1.1.1"},{"version":"2.2.2"},{"version":"3.3.3"}]}]}`,
		},
		{
			"unknown module",
			"some/random/name",
			http.StatusNotFound,
			"Not Found\n",
		},
		{
			"empty module name",
			"",
			http.StatusNotFound,
			"Not Found\n",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			req := httptest.NewRequest("GET", "/v1/modules/"+tc.module+"/versions", nil)
			w := httptest.NewRecorder()

			app.ModuleVersions().ServeHTTP(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)
			is.Equal(resp.StatusCode, tc.status)
			is.Equal(string(body), tc.body)

			if tc.status == http.StatusOK {
				is.Equal(resp.Header.Get("Content-Type"), "application/json")
			}
		})
	}
}

func TestModuleDownload(t *testing.T) {
	is := is.New(t)

	app := App{
		moduleStore: NewModuleStore(),
	}

	app.moduleStore.Set("hashicorp/consul/aws", Module{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions: []string{
			"1.1.1",
			"2.2.2",
			"3.3.3",
		},
	})

	testcases := []struct {
		name         string
		moduleString string
		status       int
		downloadURL  string
	}{
		{
			"valid module",
			"hashicorp/consul/aws/2.2.2",
			http.StatusNoContent,
			"git::ssh://git@github.com/hashicorp/consul.git?ref=2.2.2",
		},
		{
			"unknown module",
			"some/random/name/0.0.0",
			http.StatusNotFound,
			"",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			req := httptest.NewRequest("GET", "/v1/modules/"+tc.moduleString+"/download", nil)
			w := httptest.NewRecorder()

			app.ModuleDownload().ServeHTTP(w, req)

			resp := w.Result()
			is.Equal(resp.StatusCode, tc.status)
			is.Equal(resp.Header.Get("X-Terraform-Get"), tc.downloadURL) // X-Terraform-Get header

			if tc.status == http.StatusOK {
				is.Equal(resp.Header.Get("Content-Type"), "application/json")
			}
		})
	}
}
