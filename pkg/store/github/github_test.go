// SPDX-FileCopyrightText: 2022 - 2025 NRK
//
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-github/v76/github"
	"github.com/matryer/is"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

func TestGithubStore(t *testing.T) {
	t.Run("create GitHubStore", func(t *testing.T) {
		is := is.New(t)
		emptyResult := new(github.RepositoriesSearchResult)
		total := 0
		emptyResult.Total = &total
		mockedHTTPClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatch(
				mock.GetSearchRepositories,
				emptyResult,
			),
		)

		c := github.NewClient(mockedHTTPClient)
		store := &GitHubStore{
			ownerFilter: "test-owner",
			topicFilter: "test-topic",
			client:      c,
			moduleCache: make(map[string][]*core.ModuleVersion),
			logger:      zap.NewNop(),
		}

		err := store.ReloadCache(context.Background())
		is.NoErr(err)
	})

	t.Run("create GitHubStore with github error", func(t *testing.T) {
		is := is.New(t)
		mockedHTTPClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetSearchRepositories,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mock.WriteError(
						w,
						http.StatusInternalServerError,
						"github went belly up or something",
					)
				}),
			),
		)
		c := github.NewClient(mockedHTTPClient)
		store := &GitHubStore{
			ownerFilter: "test-owner",
			topicFilter: "test-topic",
			client:      c,
			moduleCache: make(map[string][]*core.ModuleVersion),
			logger:      zap.NewNop(),
		}
		store.client = c
		err := store.ReloadCache(context.Background())
		is.True(err != nil)
		ghErr, ok := err.(*github.ErrorResponse)
		if !ok {
			t.Fatal("couldn't cast userErr to *github.ErrorResponse")
		}

		if ghErr.Message != "github went belly up or something" {
			t.Errorf("user err is %s, want 'github went belly up or something'", err.Error())
		}
	})
}

func TestGetModuleVersion(t *testing.T) {
	result := new(github.RepositoriesSearchResult)
	total := 1
	result.Total = &total
	result.Repositories = []*github.Repository{
		{
			Name:     github.Ptr("testrepo"),
			FullName: github.Ptr("test-owner/test-repo"),
		},
	}
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetSearchRepositories,
			result,
		),
		mock.WithRequestMatch(
			mock.GetReposTagsByOwnerByRepo,
			[]github.RepositoryTag{
				{
					Name: github.Ptr("v1.0.0"),
				},
			},
		),
	)

	c := github.NewClient(mockedHTTPClient)
	store := &GitHubStore{
		ownerFilter: "test-owner",
		topicFilter: "test-topic",
		client:      c,
		moduleCache: make(map[string][]*core.ModuleVersion),
		logger:      zap.NewNop(),
	}

	err := store.ReloadCache(context.Background())
	if err != nil {
		t.Fatal("Could not ReloadCache")
	}

	t.Run("returns matching version", func(t *testing.T) {
		is := is.New(t)
		ver, err := store.GetModuleVersion(context.Background(), "test-owner", "test-repo", "generic", "1.0.0")
		is.True(err == nil)
		is.Equal(ver.Version, "1.0.0")
		is.Equal(ver.SourceURL, "git::ssh://git@github.com/test-owner/test-repo.git?ref=v1.0.0")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		ver, err := store.GetModuleVersion(context.Background(), "test-owner", "test-repo", "generic", "1.0.1")
		is.True(err != nil)
		is.True(ver == nil)
		is.Equal(err.Error(), "version '1.0.1' not found for module 'test-owner/test-repo/generic'")
	})

}

func TestListModuleVersions(t *testing.T) {
	result := new(github.RepositoriesSearchResult)
	total := 1
	result.Total = &total
	result.Repositories = []*github.Repository{
		{
			Name:     github.Ptr("testrepo"),
			FullName: github.Ptr("test-owner/test-repo"),
		},
	}
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetSearchRepositories,
			result,
		),
		mock.WithRequestMatch(
			mock.GetReposTagsByOwnerByRepo,
			[]github.RepositoryTag{
				{
					Name: github.Ptr("v1.0.0"),
				},
				{
					Name: github.Ptr("v1.0.1"),
				},
				{
					Name: github.Ptr("v2.0.0"),
				},
				{
					Name: github.Ptr("non-semver"),
				},
			},
		),
	)

	c := github.NewClient(mockedHTTPClient)
	store := &GitHubStore{
		ownerFilter: "test-owner",
		topicFilter: "test-topic",
		client:      c,
		moduleCache: make(map[string][]*core.ModuleVersion),
		logger:      zap.NewNop(),
	}

	err := store.ReloadCache(context.Background())
	if err != nil {
		t.Fatal("Could not ReloadCache")
	}

	t.Run("returns list of versions", func(t *testing.T) {
		is := is.New(t)
		versions, err := store.ListModuleVersions(context.Background(), "test-owner", "test-repo", "generic")
		is.True(err == nil)
		is.Equal(len(versions), 3)
		is.Equal(versions[0].Version, "1.0.0")
		is.Equal(versions[1].Version, "1.0.1")
		is.Equal(versions[2].Version, "2.0.0")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		versions, err := store.ListModuleVersions(context.Background(), "wrong", "wrong", "wrong")
		is.True(err != nil)
		is.Equal(versions, nil)
	})

}

func Test_isTransientError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "nil error",
			err:       nil,
			transient: false,
		},
		{
			name:      "generic error",
			err:       fmt.Errorf("something went wrong"),
			transient: false,
		},
		{
			name: "rate limit error",
			err: &github.RateLimitError{
				Response: &http.Response{StatusCode: 403},
				Message:  "rate limit exceeded",
			},
			transient: true,
		},
		{
			name: "abuse rate limit error",
			err: &github.AbuseRateLimitError{
				Response: &http.Response{StatusCode: 403},
				Message:  "abuse detection",
			},
			transient: true,
		},
		{
			name: "401 unauthorized",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 401,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Bad credentials",
			},
			transient: true,
		},
		{
			name: "403 forbidden",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 403,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Forbidden",
			},
			transient: true,
		},
		{
			name: "500 internal server error",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 500,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Internal Server Error",
			},
			transient: true,
		},
		{
			name: "502 bad gateway",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 502,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Bad Gateway",
			},
			transient: true,
		},
		{
			name: "404 not found - not transient",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 404,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Not Found",
			},
			transient: false,
		},
		{
			name: "422 unprocessable entity - not transient",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 422,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Validation Failed",
			},
			transient: false,
		},
		{
			name:      "wrapped transient error",
			err:       fmt.Errorf("download failed: %w", &github.ErrorResponse{Response: &http.Response{StatusCode: 401, Request: &http.Request{Method: "GET", URL: &url.URL{Path: "/test"}}}, Message: "Bad credentials"}),
			transient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTransientError(tt.err)
			if result != tt.transient {
				t.Errorf("isTransientError() = %v, want %v for error: %v", result, tt.transient, tt.err)
			}
		})
	}
}

func Test_isRateLimitError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		rateLimit bool
	}{
		{
			name:      "nil error",
			err:       nil,
			rateLimit: false,
		},
		{
			name:      "generic error",
			err:       fmt.Errorf("something went wrong"),
			rateLimit: false,
		},
		{
			name: "rate limit error",
			err: &github.RateLimitError{
				Response: &http.Response{StatusCode: 403},
				Message:  "rate limit exceeded",
			},
			rateLimit: true,
		},
		{
			name: "abuse rate limit error",
			err: &github.AbuseRateLimitError{
				Response: &http.Response{StatusCode: 403},
				Message:  "abuse detection",
			},
			rateLimit: true,
		},
		{
			name: "401 unauthorized - not a rate limit",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 401,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Bad credentials",
			},
			rateLimit: false,
		},
		{
			name: "500 server error - not a rate limit",
			err: &github.ErrorResponse{
				Response: &http.Response{
					StatusCode: 500,
					Request:    &http.Request{Method: "GET", URL: &url.URL{Path: "/repos/test"}},
				},
				Message: "Internal Server Error",
			},
			rateLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRateLimitError(tt.err)
			if result != tt.rateLimit {
				t.Errorf("isRateLimitError() = %v, want %v for error: %v", result, tt.rateLimit, tt.err)
			}
		})
	}
}

func Test_extractOsArch(t *testing.T) {
	tests := []struct {
		name   string
		args   string
		result core.Platform
		found  bool
	}{
		{"name", "terraform-provider-test_1.0.3_darwin_amd64.zip", core.Platform{OS: "darwin", Arch: "amd64"}, true},
		{"name", "terraform-provider-test_1.0.3_darwin_arm64.zip", core.Platform{OS: "darwin", Arch: "arm64"}, true},
		{"name", "terraform-provider-test_1.0.3_linux_amd64.zip", core.Platform{OS: "linux", Arch: "amd64"}, true},
		{"name", "terraform-provider-test_1.0.3_linux_arm64.zip", core.Platform{OS: "linux", Arch: "arm64"}, true},
		{"name", "terraform-provider-test_1.0.3_ugga_arm644.zip", core.Platform{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := extractOsArch(tt.args)
			if !reflect.DeepEqual(result, tt.result) {
				t.Errorf("extractOsArch() result = %v, want %v", result, tt.result)
			}
			if found != tt.found {
				t.Errorf("extractOsArch() found = %v, want %v", found, tt.found)
			}
		})
	}
}
