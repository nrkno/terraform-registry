package main

import (
	"context"
	"fmt"

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

func (c *GitHubClient) TestCredentials(ctx context.Context) error {
	// Just a random endpoint to check credentials
	_, _, err := c.client.Users.ListEmails(ctx, nil)
	return err
}

func (c *GitHubClient) ListUserRepositoriesByTopic(ctx context.Context, user string) ([]string, error) {
	var allRepos []string

	opts := &github.RepositoryListOptions{}
	opts.PerPage = 100

	for {
		repos, resp, err := c.client.Repositories.List(ctx, user, opts)
		if err != nil {
			return nil, err
		}

		for _, r := range repos {
			allRepos = append(allRepos, fmt.Sprintf("%s/%s", *r.Owner, *r.Name))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *GitHubClient) GetRepoTags(ctx context.Context, owner, repo string) ([]*github.RepositoryTag, error) {
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
