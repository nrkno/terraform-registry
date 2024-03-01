// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package github

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v59/github"
	goversion "github.com/hashicorp/go-version"
	"github.com/nrkno/terraform-registry/pkg/core"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// GitHubStore is a store implementation using GitHub as a backend.
// Should not be instantiated directly. Use `NewGitHubStore` instead.
type GitHubStore struct {
	// Org to filter repositories by. Leave empty for all.
	ownerFilter string
	// Topic to filter repositories by. Leave empty for all.
	topicFilter string

	client  *github.Client
	cache   map[string][]*core.ModuleVersion
	mut     sync.RWMutex
	metrics metrics

	logger *zap.Logger
}

func NewGitHubStore(ownerFilter, topicFilter, accessToken string, logger *zap.Logger) *GitHubStore {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	c := oauth2.NewClient(context.TODO(), ts)

	if logger == nil {
		logger = zap.NewNop()
	}

	return &GitHubStore{
		ownerFilter: ownerFilter,
		topicFilter: topicFilter,
		client:      github.NewClient(c),
		cache:       make(map[string][]*core.ModuleVersion),
		logger:      logger,
		metrics: metrics{
			rateLimitCoreLimit:             prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_core_limit"}),
			rateLimitCoreRemaining:         prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_core_remaining"}),
			rateLimitCoreResetTimestamp:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_core_reset_timestamp"}),
			rateLimitSearchLimit:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_search_limit"}),
			rateLimitSearchRemaining:       prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_search_remaining"}),
			rateLimitSearchResetTimestamp:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_search_reset_timestamp"}),
			rateLimitGraphQLLimit:          prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_graphql_limit"}),
			rateLimitGraphQLRemaining:      prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_graphql_remaining"}),
			rateLimitGraphQLResetTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{Name: "rate_limit_graphql_reset_timestamp"}),
		},
	}
}

// ListModuleVersions returns a list of module versions.
func (s *GitHubStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	versions, ok := s.cache[fmt.Sprintf("%s/%s/%s", namespace, name, provider)]
	if !ok {
		return nil, fmt.Errorf("module '%s/%s/%s' not found", namespace, name, provider)
	}

	return versions, nil
}

// GetModuleVersion returns single module version.
func (s *GitHubStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	versions, ok := s.cache[fmt.Sprintf("%s/%s/%s", namespace, name, provider)]
	if !ok {
		return nil, fmt.Errorf("module '%s/%s/%s' not found", namespace, name, provider)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version '%s' not found for module '%s/%s/%s'", version, namespace, name, provider)
}

// ReloadCache queries the GitHub API and reloads the local cache of module versions.
// Should be called at least once after initialisation and proably on regular
// intervals afterwards to keep cache up-to-date.
// This method also starts the background worker for polling rate limit status of the GitHub API on first invocation.
func (s *GitHubStore) ReloadCache(ctx context.Context) error {
	// Start rate limit worker on first invocation
	sync.OnceFunc(func() {
		go func() {
			for {
				s.updateMetrics(context.Background())
				time.Sleep(15 * time.Second)
			}
		}()
	})()

	repos, err := s.searchRepositories(ctx)
	if err != nil {
		return err
	}

	fresh := make(map[string][]*core.ModuleVersion)

	for _, repo := range repos {
		// Splitting owner from FullName to avoid getting it from GetOwner().GetName(),
		// as it seems to be empty, maybe due to missing OAuth permission scopes.
		parts := strings.Split(repo.GetFullName(), "/")
		if len(parts) != 2 {
			return fmt.Errorf("repo.FullName is not in expected format 'owner/repo', is '%s'", repo.GetFullName())
		}

		owner := parts[0]
		name := parts[1]
		key := fmt.Sprintf("%s/%s/generic", owner, name)

		tags, err := s.listAllRepoTags(ctx, owner, name)
		if err != nil {
			return err
		}

		versions := make([]*core.ModuleVersion, 0)
		for _, tag := range tags {
			version := strings.TrimPrefix(tag.GetName(), "v") // Terraform uses SemVer names without 'v' prefix
			if _, err := goversion.NewSemver(version); err == nil {
				versions = append(versions, &core.ModuleVersion{
					Version:   version,
					SourceURL: fmt.Sprintf("git::ssh://git@github.com/%s/%s.git?ref=%s", owner, name, tag.GetName()),
				})
			}
		}

		s.logger.Debug("found module",
			zap.String("name", key),
			zap.Int("version_count", len(versions)),
		)

		fresh[key] = versions
	}

	// This cleans up modules that are no longer available and
	// reduces write lock duration by not modifying the cache directly
	// on each iteration.
	s.mut.Lock()
	s.cache = fresh
	s.mut.Unlock()

	return nil
}

// listAllRepoTags lists all tags for the specified repository.
// When an error is returned, the tags fetched up until the point of error
// is also returned.
func (s *GitHubStore) listAllRepoTags(ctx context.Context, owner, repo string) ([]*github.RepositoryTag, error) {
	var allTags []*github.RepositoryTag

	opts := &github.ListOptions{PerPage: 100}

	for {
		tags, resp, err := s.client.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return allTags, err
		}

		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}

// searchRepositories fetches all repositories matching the configured filters.
// When an error is returned, the repositories fetched up until the point of error
// is also returned.
func (s *GitHubStore) searchRepositories(ctx context.Context) ([]*github.Repository, error) {
	var (
		allRepos []*github.Repository
		filters  []string
	)

	if s.ownerFilter != "" {
		filters = append(filters, fmt.Sprintf(`org:"%s"`, s.ownerFilter))
	}
	if s.topicFilter != "" {
		filters = append(filters, fmt.Sprintf(`topic:"%s"`, s.topicFilter))
	}

	opts := &github.SearchOptions{}
	opts.ListOptions.PerPage = 100

	for {
		result, resp, err := s.client.Search.Repositories(ctx, strings.Join(filters, " "), opts)
		if err != nil {
			return allRepos, err
		}

		allRepos = append(allRepos, result.Repositories...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

type metrics struct {
	rateLimitCoreLimit             prometheus.Gauge
	rateLimitCoreRemaining         prometheus.Gauge
	rateLimitCoreResetTimestamp    prometheus.Gauge
	rateLimitSearchLimit           prometheus.Gauge
	rateLimitSearchRemaining       prometheus.Gauge
	rateLimitSearchResetTimestamp  prometheus.Gauge
	rateLimitGraphQLLimit          prometheus.Gauge
	rateLimitGraphQLRemaining      prometheus.Gauge
	rateLimitGraphQLResetTimestamp prometheus.Gauge
}

// updateMetrics updates all metrics that needs polling.
func (s *GitHubStore) updateMetrics(ctx context.Context) {
	s.logger.Debug("refreshing metrics")

	ratel, _, err := s.client.RateLimit.Get(ctx)
	if err != nil {
		s.logger.Warn("failed to get rate limit status", zap.Errors("err", []error{err}))
	} else {
		s.metrics.rateLimitCoreLimit.Set(float64(ratel.Core.Limit))
		s.metrics.rateLimitCoreRemaining.Set(float64(ratel.Core.Remaining))
		s.metrics.rateLimitCoreResetTimestamp.Set(float64(ratel.Core.Reset.Unix()))
		s.metrics.rateLimitSearchLimit.Set(float64(ratel.Search.Limit))
		s.metrics.rateLimitSearchRemaining.Set(float64(ratel.Search.Remaining))
		s.metrics.rateLimitSearchResetTimestamp.Set(float64(ratel.Search.Reset.Unix()))
		s.metrics.rateLimitGraphQLLimit.Set(float64(ratel.GraphQL.Limit))
		s.metrics.rateLimitGraphQLRemaining.Set(float64(ratel.GraphQL.Remaining))
		s.metrics.rateLimitGraphQLResetTimestamp.Set(float64(ratel.GraphQL.Reset.Unix()))
	}
}

// Metrics returns a registry with metrics for this store.
func (s *GitHubStore) Metrics() prometheus.Collector {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		s.metrics.rateLimitCoreLimit,
		s.metrics.rateLimitCoreRemaining,
		s.metrics.rateLimitCoreResetTimestamp,
		s.metrics.rateLimitSearchLimit,
		s.metrics.rateLimitSearchRemaining,
		s.metrics.rateLimitSearchResetTimestamp,
		s.metrics.rateLimitGraphQLLimit,
		s.metrics.rateLimitGraphQLRemaining,
		s.metrics.rateLimitGraphQLResetTimestamp,
	)
	return reg
}
