// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package registry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/matryer/is"
	"github.com/nrkno/terraform-registry/pkg/core"
	memstore "github.com/nrkno/terraform-registry/pkg/store/memory"
	"go.uber.org/zap"
)

func verifyServiceDiscovery(t *testing.T, resp *http.Response) {
	is := is.New(t)

	body, _ := io.ReadAll(resp.Body)

	is.Equal(resp.StatusCode, 200)
	is.Equal(resp.Header.Get("Content-Type"), "application/json")
	is.True(len(body) > 1)

	var compactJSON bytes.Buffer
	err := json.Compact(&compactJSON, body)
	is.NoErr(err)

	is.Equal(
		compactJSON.String(),
		`{"modules.v1":"/v1/modules/","providers.v1":"/v1/providers/"}`,
	)
}

func TestServiceDiscovery(t *testing.T) {
	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()

	reg := Registry{
		IsAuthDisabled: true,
		logger:         zap.NewNop(),
	}
	reg.setupRoutes()
	reg.router.ServeHTTP(w, req)

	resp := w.Result()
	verifyServiceDiscovery(t, resp)
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
			authTokens: map[string]string{
				"description": authToken,
			},
			logger: zap.NewNop(),
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

func verifyHealth(t *testing.T, resp *http.Response, expectedStatus int, expectedResponse HealthResponse) {
	is := is.New(t)

	body, err := io.ReadAll(resp.Body)
	is.NoErr(err)
	is.Equal(resp.StatusCode, expectedStatus)
	is.Equal(resp.Header.Get("Content-Type"), "application/json")

	var respObj HealthResponse
	err = json.Unmarshal(body, &respObj)
	is.NoErr(err)

	is.Equal(respObj, expectedResponse)
}

func TestHealth(t *testing.T) {
	mstore := memstore.NewMemoryStore()
	reg := Registry{
		IsAuthDisabled: true,
		moduleStore:    mstore,
		logger:         zap.NewNop(),
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
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			reg.router.ServeHTTP(w, req)

			resp := w.Result()
			verifyHealth(t, resp, tc.statusCode, tc.health)
		})
	}
}

func verifyModuleVersions(t *testing.T, resp *http.Response, expectedStatus int, expectedVersion []string) {
	is := is.New(t)
	body, err := io.ReadAll(resp.Body)
	is.NoErr(err)
	is.Equal(resp.StatusCode, expectedStatus)

	if expectedStatus == http.StatusOK {
		is.Equal(resp.Header.Get("Content-Type"), "application/json")

		var respObj ModuleVersionsResponse
		err := json.Unmarshal(body, &respObj)
		is.NoErr(err)

		var actualVersions []string
		for _, v := range respObj.Modules[0].Versions {
			actualVersions = append(actualVersions, v.Version)
		}
		sort.Strings(actualVersions)

		is.Equal(actualVersions, expectedVersion)
	}
}

func TestListModuleVersions(t *testing.T) {
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
		logger:         zap.NewNop(),
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
			req := httptest.NewRequest("GET", "/v1/modules/"+tc.module+"/versions", nil)
			w := httptest.NewRecorder()

			reg.router.ServeHTTP(w, req)

			resp := w.Result()
			verifyModuleVersions(t, resp, tc.status, tc.versionsSeen)
		})
	}
}

func verifyDownload(t *testing.T, resp *http.Response, expectedStatus int, expectedURL string) {
	is := is.New(t)
	is.Equal(resp.StatusCode, expectedStatus)
	is.Equal(resp.Header.Get("X-Terraform-Get"), expectedURL) // X-Terraform-Get header

	if expectedStatus == http.StatusOK {
		is.Equal(resp.Header.Get("Content-Type"), "application/json")
	}
}

func TestModuleDownload(t *testing.T) {
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
		logger:         zap.NewNop(),
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
			req := httptest.NewRequest("GET", "/v1/modules/"+tc.moduleString+"/download", nil)
			w := httptest.NewRecorder()

			reg.router.ServeHTTP(w, req)

			resp := w.Result()
			verifyDownload(t, resp, tc.status, tc.downloadURL)
		})
	}
}

func setupTestRegistry() *Registry {
	mstore := memstore.NewMemoryStore()
	mstore.Set("hashicorp/consul/aws", []*core.ModuleVersion{
		&core.ModuleVersion{
			Version:   "2.2.2",
			SourceURL: "git::ssh://git@github.com/hashicorp/consul.git?ref=v2.2.2",
		},
	})

	reg := &Registry{
		IsAuthDisabled: false,
		moduleStore:    mstore,
		logger:         zap.NewNop(),
	}
	reg.setupRoutes()

	return reg
}

func verifyRoute(t *testing.T, resp *http.Response, path string, authenticated bool) {
	is := is.New(t)
	url, err := url.Parse(path)
	if err != nil {
		return
	}
	healthUrl := regexp.MustCompile("^/health($|[?].*)")
	wellknownUrl := regexp.MustCompile(`^/\.well-known/terraform\.json($|[?].*)`)
	moduleDownloadRoute := regexp.MustCompile("^/v1/modules/[^/]+/[^/]+/[^/]+/[^/]+/download($|[?].*)")
	moduleVersionRoute := regexp.MustCompile("^/v1/modules/[^/]+/[^/]+/[^/]+/versions($|[?].*)")
	providerDownloadRoute := regexp.MustCompile("^/v1/providers/[^/]+/[^/]+/versions($|[?].*)")
	providerVersionRoute := regexp.MustCompile("^/v1/providers/[^/]+/[^/]+/[^/]+/download/[^/]+/[^/]+($|[?].*)")
	providerDownloadAssetRoute := regexp.MustCompile("^/download/provider[^/]+/[^/]+/[^/]+/assets($|[?].*)")
	switch {
	case path == "/":
		t.Logf("Checking index path '%s', parsed path is '%s'", path, url.Path)
		is.Equal(resp.StatusCode, http.StatusOK)
	case healthUrl.MatchString(path):
		t.Logf("Checking health, path '%s'", path)
		verifyHealth(t, resp, http.StatusOK, HealthResponse{
			Status: "OK",
		})
	case wellknownUrl.MatchString(path):
		t.Logf("Checking well known, path '%s'", path)
		verifyServiceDiscovery(t, resp)
	case authenticated && moduleVersionRoute.MatchString(path):
		t.Logf("Checking module version, path '%s'", path)
		if strings.HasPrefix(path, "/v1/modules/hashicorp/consul/aws/versions") {
			verifyModuleVersions(t, resp, http.StatusOK, []string{"2.2.2"})
		} else {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		}
	case authenticated && moduleDownloadRoute.MatchString(path):
		t.Logf("Checking module download, path '%s'", path)
		if strings.HasPrefix(path, "/v1/modules/hashicorp/consul/aws/2.2.2/download") {
			verifyDownload(t, resp, http.StatusNoContent, "git::ssh://git@github.com/hashicorp/consul.git?ref=v2.2.2")
		} else {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		}
	case authenticated && providerVersionRoute.MatchString(path):
		t.Logf("Checking provider version, path '%s'", path)
		if strings.HasPrefix(path, "/v1/providers/hashicorp/aws/versions") {
			verifyModuleVersions(t, resp, http.StatusOK, []string{"2.2.2"})
		} else {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		}
	case authenticated && providerDownloadRoute.MatchString(path):
		t.Logf("Checking provider download, path '%s'", path)
		is.Equal(resp.StatusCode, http.StatusNotFound)

	case authenticated && providerDownloadAssetRoute.MatchString(path):
		t.Logf("Checking provider asset download, path '%s'", path)
		is.Equal(resp.StatusCode, http.StatusNotFound)

	case authenticated && (url.Path == "/v1" || strings.HasPrefix(url.Path, "/v1/")):
		t.Logf("Checking authenticated v1, path '%s'", path)
		t.Logf("Response is '%v'", resp.StatusCode)
		t.Logf("authenticated %v", authenticated)
		is.Equal(resp.StatusCode, http.StatusNotFound)
	case url.Path == "/v1" || strings.HasPrefix(url.Path, "/v1/"):
		//case v1Url.MatchString(path):
		t.Logf("Checking unathenticated v1, path '%s', parsed path is '%s'", path, url.Path)
		t.Logf("Fragment is '%v'", url.Fragment)
		t.Logf("Response is '%v'", resp.StatusCode)
		t.Logf("authenticated %v", authenticated)
		if path == "/v1#" {
			is.Equal(resp.StatusCode, http.StatusNotFound)
			// for instance //0/v1 is parsed as /v1
		} else if !strings.HasPrefix(path, "/v1") {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		} else if strings.HasPrefix(path, "/v1#") {
			is.Equal(resp.StatusCode, http.StatusNotFound)
		} else {
			is.Equal(resp.StatusCode, http.StatusForbidden)
		}
	default:
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Is no match for url '%s'", path)
		t.Logf("Parsed path is '%s'", url.Path)
		t.Logf("Frament is '%s'", url.Fragment)
		t.Logf("authenticated %v", authenticated)
		t.Logf("Headers is %v", resp.Header)
		t.Logf("Body is '%v'", string(body))
		is.Equal(resp.StatusCode, http.StatusNotFound)
	}
}

func FuzzRoutes(f *testing.F) {
	for _, seed := range []string{
		"/",
		"/health",
		"/.well-known/terraform.json",
		"/v1/modules/hashicorp/consul/aws/versions",
		"/v1/modules/hashicorp/consul/aws/2.2.2/download",
		"/v1/modules/does/not/exist/versions",
		"/v1/providers/hashicorp/aws/versions",
		"/v1/providers/hashicorp/aws/2.2.2/download/darwin/arm64",
		"/v1/providers/does/not/exist/versions",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, url string) {
		defer func() {
			recover()
		}()

		reg := setupTestRegistry()
		reg.SetAuthTokens(map[string]string{
			"foo": "testauth",
		})

		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		reg.router.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		verifyRoute(t, resp, url, false)

		req = httptest.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer testauth")
		w = httptest.NewRecorder()
		reg.router.ServeHTTP(w, req)

		resp = w.Result()
		defer resp.Body.Close()
		verifyRoute(t, resp, url, true)
	})
}
