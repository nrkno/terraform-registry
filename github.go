package main

import (
	"context"
	"fmt"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
)

// GitHubClient should not be initialised directly. Use `NewGitHubClient` instead.
type GitHubClient struct {
	client *github.Client
}

func NewGitHubClient(accessToken string) *GitHubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	c := oauth2.NewClient(context.TODO(), ts)
	return &GitHubClient{
		client: github.NewClient(c),
	}
}

// TestCredentials checks if the credentials are valid by attempting to read the
// token owner's email addresses. This may not be an accurate test, due to different
// auth scopes being available, but at least verifies that it's a valid token.
func (c *GitHubClient) TestCredentials(ctx context.Context) error {
	// Just a random endpoint to check credentials
	_, _, err := c.client.Users.ListEmails(ctx, nil)
	return err
}

// ListAllUserRepositories lists all repositories for the specified user/owner.
func (c *GitHubClient) ListAllUserRepositories(ctx context.Context, user string) ([]string, error) {
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

// ListAllRepoTags lists all tags for for the specified repository.
func (c *GitHubClient) ListAllRepoTags(ctx context.Context, owner, repo string) ([]*github.RepositoryTag, error) {
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
