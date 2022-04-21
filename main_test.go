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

	req := httptest.NewRequest("GET", "/.well-know/terraform.json", nil)

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
		Router: mux.NewRouter(),
		authTokens: []string{
			"valid",
		},
	}
	app.Router.Use(app.TokenAuth)
	app.Router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

			app.Router.ServeHTTP(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			is.NoErr(err)

			is.Equal(resp.StatusCode, tc.status)
			is.Equal(string(body), tc.body)
		})
	}
}

//func TestModuleVersions(t *testing.T) {
//	is := is.New(t)
//
//	req := httptest.NewRequest("GET", "/", nil)
//
//	w := httptest.NewRecorder()
//	ServiceDiscovery(w, req)
//
//	resp := w.Result()
//	body, _ := io.ReadAll(resp.Body)
//
//	is.Equal(resp.StatusCode, 200)
//	is.Equal(resp.Header.Get("Content-Type"), "application/json")
//	is.True(len(body) > 1)
//
//	err := json.Unmarshal(body, &struct{}{})
//	is.NoErr(err)
//}
