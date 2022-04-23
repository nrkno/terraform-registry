package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/matryer/is"
)

func TestServiceDiscovery(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()

	app := App{
		IsAuthDisabled: true,
	}
	app.SetupRouter()
	app.router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	is.Equal(resp.StatusCode, 200)
	is.Equal(resp.Header.Get("Content-Type"), "application/json")
	is.True(len(body) > 1)

	var compactJSON bytes.Buffer
	err := json.Compact(&compactJSON, body)
	is.NoErr(err)

	is.Equal(
		compactJSON.String(),
		`{"modules.v1":"/v1/modules/","login.v1":{"client":"terraform-cli","grant_types":["authz_code"],"authz":"/oauth/authorization","token":"/oauth/token","ports":[10000,10010]}}`,
	)

}

func TestTokenAuth(t *testing.T) {
	is := is.New(t)

	app := App{
		authTokens: []string{
			"valid",
		},
	}
	app.SetupRouter()

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
			"Terraform Registry\n",
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
		IsAuthDisabled: true,
		moduleStore:    NewModuleStore(),
	}
	app.SetupRouter()

	versions := make(map[string]string)
	versions["1.1.1"] = "v1.1.1"
	versions["2.2.2"] = "v2.2.2"
	versions["3.3.3"] = "v3.3.3"
	app.moduleStore.Set("hashicorp/consul/aws", Module{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions:  versions,
	})

	testcases := []struct {
		name         string
		module       string
		status       int
		versionsSeen []string
	}{
		{
			"valid module",
			"hashicorp/consul/aws",
			http.StatusOK,
			[]string{"1.1.1", "2.2.2", "3.3.3"},
		},
		{
			"unknown module",
			"some/random/name",
			http.StatusNotFound,
			[]string{},
		},
		{
			"empty module name",
			"",
			http.StatusNotFound,
			[]string{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			req := httptest.NewRequest("GET", "/v1/modules/"+tc.module+"/versions", nil)
			w := httptest.NewRecorder()

			app.router.ServeHTTP(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)
			is.Equal(resp.StatusCode, tc.status)

			if tc.status == http.StatusOK {
				is.Equal(resp.Header.Get("Content-Type"), "application/json")

				var respObj ModuleVersionsResponse
				err := json.Unmarshal(body, &respObj)
				is.NoErr(err)

				var actualVersions []string
				for _, v := range respObj.Modules[0].Versions {
					actualVersions = append(actualVersions, v.Version)
				}
				sort.Strings(actualVersions)

				is.Equal(actualVersions, tc.versionsSeen)
			}
		})
	}
}

func TestModuleDownload(t *testing.T) {
	is := is.New(t)

	app := App{
		IsAuthDisabled: true,
		moduleStore:    NewModuleStore(),
	}
	app.SetupRouter()

	versions := make(map[string]string)
	versions["1.1.1"] = "v1.1.1"
	versions["2.2.2"] = "v2.2.2"
	versions["3.3.3"] = "v3.3.3"
	app.moduleStore.Set("hashicorp/consul/aws", Module{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions:  versions,
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
			"git::ssh://git@github.com/hashicorp/consul.git?ref=v2.2.2",
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

			app.router.ServeHTTP(w, req)

			resp := w.Result()
			is.Equal(resp.StatusCode, tc.status)
			is.Equal(resp.Header.Get("X-Terraform-Get"), tc.downloadURL) // X-Terraform-Get header

			if tc.status == http.StatusOK {
				is.Equal(resp.Header.Get("Content-Type"), "application/json")
			}
		})
	}
}
