// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package registry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/matryer/is"
	"github.com/nrkno/terraform-registry/pkg/core"
	memstore "github.com/nrkno/terraform-registry/pkg/store/memory"
)

func TestServiceDiscovery(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()

	reg := Registry{
		IsAuthDisabled: true,
	}
	reg.setupRoutes()
	reg.router.ServeHTTP(w, req)

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

func FuzzTokenAuth(f *testing.F) {
	seeds := []struct {
		authToken           string
		authorizationHeader string
	}{
		{
			"valid",
			"Bearer valid",
		},
		{
			"valid",
			"Bearer invalid",
		},
		{
			"valid",
			"notvalid",
		},
	}
	for _, seed := range seeds {
		f.Add(seed.authToken, seed.authorizationHeader)
	}
	f.Fuzz(func(t *testing.T, authToken string, authorizationHeader string) {
		reg := Registry{
			authTokens: []string{
				authToken,
			},
		}
		reg.setupRoutes()

		is := is.New(t)

		req := httptest.NewRequest("GET", "/v1/", nil)
		req.Header.Set("Authorization", authorizationHeader)
		w := httptest.NewRecorder()

		reg.router.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		if "Bearer "+authToken == authorizationHeader {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		} else {
			is.Equal(resp.StatusCode, http.StatusForbidden)
		}
	})
}

func TestHealth(t *testing.T) {
	is := is.New(t)

	mstore := memstore.NewMemoryStore()
	reg := Registry{
		IsAuthDisabled: true,
		moduleStore:    mstore,
	}
	reg.setupRoutes()

	testcases := []struct {
		name       string
		statusCode int
		health     HealthResponse
	}{
		{
			"healthy",
			http.StatusOK,
			HealthResponse{
				Status: "OK",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			reg.router.ServeHTTP(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)
			is.Equal(resp.StatusCode, tc.statusCode)
			is.Equal(resp.Header.Get("Content-Type"), "application/json")

			var respObj HealthResponse
			err = json.Unmarshal(body, &respObj)
			is.NoErr(err)

			is.Equal(respObj, tc.health)
		})
	}
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)

	mstore := memstore.NewMemoryStore()
	mstore.Set("hashicorp/consul/aws", []*core.ModuleVersion{
		&core.ModuleVersion{
			Version: "1.1.1",
		},
		&core.ModuleVersion{
			Version: "2.2.2",
		},
		&core.ModuleVersion{
			Version: "3.3.3",
		},
	})

	reg := Registry{
		IsAuthDisabled: true,
		moduleStore:    mstore,
	}
	reg.setupRoutes()

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

			reg.router.ServeHTTP(w, req)

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

	mstore := memstore.NewMemoryStore()
	mstore.Set("hashicorp/consul/aws", []*core.ModuleVersion{
		&core.ModuleVersion{
			Version:   "1.1.1",
			SourceURL: "git::ssh://git@github.com/hashicorp/consul.git?ref=v1.1.1",
		},
		&core.ModuleVersion{
			Version:   "2.2.2",
			SourceURL: "git::ssh://git@github.com/hashicorp/consul.git?ref=v2.2.2",
		},
		&core.ModuleVersion{
			Version:   "3.3.3",
			SourceURL: "git::ssh://git@github.com/hashicorp/consul.git?ref=v3.3.3",
		},
	})

	reg := Registry{
		IsAuthDisabled: true,
		moduleStore:    mstore,
	}
	reg.setupRoutes()

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

			reg.router.ServeHTTP(w, req)

			resp := w.Result()
			is.Equal(resp.StatusCode, tc.status)
			is.Equal(resp.Header.Get("X-Terraform-Get"), tc.downloadURL) // X-Terraform-Get header

			if tc.status == http.StatusOK {
				is.Equal(resp.Header.Get("Content-Type"), "application/json")
			}
		})
	}
}
