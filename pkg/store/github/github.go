// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v43/github"
	"github.com/nrkno/terraform-registry/pkg/core"
	"golang.org/x/oauth2"
)

type GitHubStore struct {
	OrgName string
	client  *github.Client
}

func NewGitHubStore(orgName, accessToken string) *GitHubStore {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	c := oauth2.NewClient(context.TODO(), ts)

	return &GitHubStore{
		OrgName: orgName,
		client:  github.NewClient(c),
	}
}

func (c *GitHubStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	tags, err := c.listAllRepoTags(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	var vers []*core.ModuleVersion
	for _, t := range tags {
		vers = append(vers, &core.ModuleVersion{
			Version:   strings.TrimPrefix(t.GetName(), "v"),
			SourceURL: fmt.Sprintf("git::ssh://git@github.com/%s/%s.git?ref=%s", namespace, name, t.GetName()),
		})
	}
	return vers, nil
}

func (c *GitHubStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	versions, err := c.ListModuleVersions(ctx, namespace, name, provider)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version '%s' not found for module '%s/%s/%s'", version, namespace, name, provider)
}

// listAllRepoTags lists all tags for for the specified repository.
func (c *GitHubStore) listAllRepoTags(ctx context.Context, owner, repo string) ([]*github.RepositoryTag, error) {
	var allTags []*github.RepositoryTag

	opts := &github.ListOptions{PerPage: 100}

	for {
		tags, resp, err := c.client.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}

		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}
