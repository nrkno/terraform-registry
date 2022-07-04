// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v43/github"
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
			cache:       make(map[string][]*core.ModuleVersion),
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
			cache:       make(map[string][]*core.ModuleVersion),
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
			Name:     github.String("testrepo"),
			FullName: github.String("test-owner/test-repo"),
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
					Name: github.String("v1.0.0"),
				},
			},
		),
	)

	c := github.NewClient(mockedHTTPClient)
	store := &GitHubStore{
		ownerFilter: "test-owner",
		topicFilter: "test-topic",
		client:      c,
		cache:       make(map[string][]*core.ModuleVersion),
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
			Name:     github.String("testrepo"),
			FullName: github.String("test-owner/test-repo"),
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
					Name: github.String("v1.0.0"),
				},
				{
					Name: github.String("v1.0.1"),
				},
				{
					Name: github.String("v2.0.0"),
				},
				{
					Name: github.String("non-semver"),
				},
			},
		),
	)

	c := github.NewClient(mockedHTTPClient)
	store := &GitHubStore{
		ownerFilter: "test-owner",
		topicFilter: "test-topic",
		client:      c,
		cache:       make(map[string][]*core.ModuleVersion),
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
