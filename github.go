package main

import (
	"context"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client *github.Client
}

func NewGitHubClient(accessToken string) *GitHubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(context.TODO(), ts)
	return &GitHubClient{
		client: github.NewClient(tc),
	}
}

func (c *GitHubClient) GetRepoTags(ctx context.Context, owner, repo string) ([]string, error) {
	var allTags []string

	opts := &github.ListOptions{PerPage: 100}

	for {
		tags, resp, err := c.client.Repositories.ListTags(ctx, owner, repo, opt)
		if err != nil {
			return err
		}

		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allTags, nil
}
