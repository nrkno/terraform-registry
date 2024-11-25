// SPDX-FileCopyrightText: 2022 - 2024 NRK
//
// SPDX-License-Identifier: MIT

package github

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/go-github/v43/github"
	goversion "github.com/hashicorp/go-version"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var (
	releaseRegex = regexp.MustCompile(`_(freebsd|darwin|linux|windows)_([a-zA-Z0-9]+)\.+`)
)

type SHASum struct {
	Hash     string
	FileName string
}

func parseSHASumsFile(r io.Reader) map[string]string {
	sums := make(map[string]string)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		hash := parts[0]
		fileName := parts[1]

		sums[fileName] = hash
	}
	return sums
}

// GitHubStore is a store implementation using GitHub as a backend.
// Should not be instantiated directly. Use `NewGitHubStore` instead.
type GitHubStore struct {
	// Org to filter repositories by. Leave empty for all.
	ownerFilter string
	// Topic to filter repositories by. Leave empty for all.
	topicFilter string
	// Topic to filter provider repositories by. Leave empty for all.
	providerOwnerFilter string
	// Topic to filter provider repositories by. Leave empty for all.
	providerTopicFilter string

	client                *github.Client
	moduleCache           map[string][]*core.ModuleVersion
	providerVersionsCache map[string]*core.ProviderVersions
	providerCache         map[string]*core.Provider
	providerIgnoreCache   sync.Map
	moduleMut             sync.RWMutex
	providerMut           sync.RWMutex

	logger *zap.Logger
}

func NewGitHubStore(ownerFilter, topicFilter, providerOwnerFilter, providerTopicFilter, accessToken string, logger *zap.Logger) *GitHubStore {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	c := oauth2.NewClient(context.TODO(), ts)

	if logger == nil {
		logger = zap.NewNop()
	}

	return &GitHubStore{
		ownerFilter:           ownerFilter,
		topicFilter:           topicFilter,
		providerOwnerFilter:   providerOwnerFilter,
		providerTopicFilter:   providerTopicFilter,
		client:                github.NewClient(c),
		moduleCache:           make(map[string][]*core.ModuleVersion),
		providerVersionsCache: make(map[string]*core.ProviderVersions),
		providerCache:         make(map[string]*core.Provider),
		logger:                logger,
	}
}

// ListModuleVersions returns a list of module versions.
func (s *GitHubStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	s.moduleMut.RLock()
	defer s.moduleMut.RUnlock()

	key := cacheKey(namespace, name, provider)
	versions, ok := s.moduleCache[key]
	if !ok {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	return versions, nil
}

// GetModuleVersion returns single module version.
func (s *GitHubStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	s.moduleMut.RLock()
	defer s.moduleMut.RUnlock()

	key := cacheKey(namespace, name, provider)
	versions, ok := s.moduleCache[key]
	if !ok {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version '%s' not found for module '%s'", version, key)
}

func (s *GitHubStore) ListProviderVersions(ctx context.Context, namespace string, name string) (*core.ProviderVersions, error) {
	s.providerMut.RLock()
	defer s.providerMut.RUnlock()

	key := cacheKey(namespace, name)
	versions, ok := s.providerVersionsCache[key]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", key)
	}

	return versions, nil
}

func (s *GitHubStore) GetProviderVersion(ctx context.Context, namespace string, name string, version string, os string, arch string) (*core.Provider, error) {
	s.providerMut.RLock()
	defer s.providerMut.RUnlock()

	key := cacheKey(namespace, name, version, os, arch)
	provider, ok := s.providerCache[key]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", key)
	}

	return provider, nil
}

func (s *GitHubStore) GetProviderAsset(ctx context.Context, owner string, repo string, tag string, assetName string) (io.ReadCloser, error) {
	nameKey := strings.TrimPrefix(repo, "terraform-provider-")
	key := cacheKey(owner, nameKey)
	versions, ok := s.providerVersionsCache[key]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", key)
	}

	found := false
	for _, version := range versions.Versions {
		if strings.TrimPrefix(tag, "v") == version.Version {
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("provider version '%s' not found", tag)
	}

	releases, _, err := s.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, err
	}

	asset, err := s.findAsset(ctx, owner, repo, releases, assetName)
	if err != nil {
		return nil, err
	}

	return asset, nil
}

func (s *GitHubStore) findAsset(ctx context.Context, owner string, repo string, byTag *github.RepositoryRelease, assetName string) (io.ReadCloser, error) {
	var (
		err          error
		releaseAsset io.ReadCloser
	)

	for _, asset := range byTag.Assets {
		if asset.GetName() == assetName {
			releaseAsset, _, err = s.client.Repositories.DownloadReleaseAsset(ctx, owner, repo, asset.GetID(), http.DefaultClient)
			if err != nil {
				s.logger.Error(err.Error())
				return nil, fmt.Errorf("error getting asset: %s", err)
			} else {
				break
			}
		}
	}
	return releaseAsset, nil
}

// ReloadProviderCache queries the GitHub API and reloads the local providerCache of provider versions.
// Should be called at least once after initialisation and probably on regular
// intervals afterward to keep providerCache up-to-date.
func (s *GitHubStore) ReloadProviderCache(ctx context.Context) error {
	repos, err := s.searchProviderRepositories(ctx)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		s.logger.Warn("could not find any provider repos matching filter",
			zap.String("topic", s.providerTopicFilter),
			zap.String("owner", s.providerOwnerFilter))
	}

	providerVersionsCache := make(map[string]*core.ProviderVersions)
	providerCache := make(map[string]*core.Provider)

	for _, repo := range repos {
		owner, name, err := getOwnerRepoName(repo)
		if err != nil {
			return err
		}

		// HashiCorp (and thus we) require that all provider repositories must match the pattern
		// terraform-provider-{NAME}. Only lowercase repository names are supported.
		if !strings.HasPrefix(name, "terraform-provider-") {
			continue
		}
		nameKey := strings.TrimPrefix(name, "terraform-provider-")

		start := time.Now()
		releases, err := s.listAllRepoReleases(ctx, owner, name)
		if err != nil {
			return err
		}

		var versions []core.ProviderVersion
		for _, release := range releases {
			var platforms []core.Platform
			version := strings.TrimPrefix(release.GetName(), "v")

			if _, ok := s.providerIgnoreCache.Load(cacheKey(nameKey, version)); ok {
				s.logger.Debug(fmt.Sprintf("ignoring release [%s/%s], previously found to be not valid", nameKey, version))
				continue
			}

			SHASums, SHASumURL, SHASumFileName, err := s.getSHA256Sums(ctx, owner, name, release.Assets)
			if err != nil {
				s.logger.Warn(fmt.Sprintf("not a valid release [%s/%s]- could not find SHA checksums: %s", nameKey, version, err))
				s.providerIgnoreCache.Store(cacheKey(nameKey, version), true)
				continue
			}

			// not considered a valid release if a shasum file was not part of the release
			if SHASumURL == "" {
				s.logger.Warn(fmt.Sprintf("not a valid release [%s/%s] - could not find SHA checksums", nameKey, version))
				s.providerIgnoreCache.Store(cacheKey(nameKey, version), true)
				continue
			}

			providerProtocols, err := s.getProviderProtocols(ctx, owner, name, release.Assets)
			if err != nil {
				s.logger.Warn(fmt.Sprintf("not a valid release [%s/%s] - unable to identify provider protocol", nameKey, version))
				s.providerIgnoreCache.Store(cacheKey(nameKey, version), true)
				continue
			}

			keys, err := s.getGPGPublicKey(ctx, release, owner, name)
			if err != nil || len(keys) != 1 {
				s.logger.Warn(fmt.Sprintf("not a valid release [%s/%s] - unable to get GPG Public Key", nameKey, version))
				s.providerIgnoreCache.Store(cacheKey(nameKey, version), true)
				continue
			}

			for _, asset := range release.Assets {
				platform, ok := extractOsArch(asset.GetName())

				// if asset does not contain os/arch info, it is not a provider binary
				if !ok {
					continue
				}

				platforms = append(platforms, platform)

				downloadUrl := asset.GetBrowserDownloadURL()
				SHASumSigURL := SHASumURL + ".sig"
				if repo.GetPrivate() {
					downloadUrl = fmt.Sprintf("/download/provider/%s/%s/v%s/asset/%s", owner, name, version, asset.GetName())
					SHASumURL = fmt.Sprintf("/download/provider/%s/%s/v%s/asset/%s", owner, name, version, SHASumFileName)
					SHASumSigURL = fmt.Sprintf("/download/provider/%s/%s/v%s/asset/%s", owner, name, version, SHASumFileName+".sig")
				}

				p := &core.Provider{
					Protocols:           providerProtocols,
					OS:                  platform.OS,
					Arch:                platform.Arch,
					Filename:            asset.GetName(),
					DownloadURL:         downloadUrl,
					SHASumsURL:          SHASumURL,
					SHASumsSignatureURL: SHASumSigURL,
					SHASum:              SHASums[asset.GetName()],
					SigningKeys:         core.SigningKeys{GPGPublicKeys: keys},
				}

				// update the fresh providerCache
				providerCache[cacheKey(owner, nameKey, version, platform.OS, platform.Arch)] = p
			}

			if len(platforms) > 0 {
				pv := core.ProviderVersion{
					Version:   version,
					Protocols: providerProtocols,
					Platforms: platforms,
				}
				versions = append(versions, pv)
			}
		}

		duration := time.Since(start)
		s.logger.Debug("found provider",
			zap.String("name", fmt.Sprintf("%s/%s", owner, nameKey)),
			zap.Int("versions", len(versions)),
			zap.Duration("duration", duration),
		)

		// update the fresh providerVersionCache
		providerVersionsCache[cacheKey(owner, nameKey)] = &core.ProviderVersions{Versions: versions}
	}

	// This cleans up modules that are no longer available and
	// reduces write lock duration by not modifying the caches directly
	// on each iteration.
	s.providerMut.Lock()
	s.providerCache = providerCache
	s.providerVersionsCache = providerVersionsCache
	s.providerMut.Unlock()

	return nil
}

func (s *GitHubStore) getGPGPublicKey(ctx context.Context, release *github.RepositoryRelease, owner string, name string) ([]core.GpgPublicKeys, error) {
	var keys []core.GpgPublicKeys
	for _, asset := range release.Assets {
		if strings.Contains(asset.GetName(), "gpg-public-key.pem") {
			releaseAsset, _, err := s.client.Repositories.DownloadReleaseAsset(ctx, owner, name, asset.GetID(), http.DefaultClient)
			if err != nil {
				return nil, err
			}

			all, err := io.ReadAll(releaseAsset)
			releaseAsset.Close()
			if err != nil {
				return nil, err
			}

			els, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(all))
			if err != nil {
				return nil, err
			}

			if len(els) != 1 {
				return nil, fmt.Errorf("GPG Key contains %d entities, wanted 1", len(els))
			}

			key := els[0]
			keys = []core.GpgPublicKeys{
				{KeyID: key.PrimaryKey.KeyIdString(), ASCIIArmor: string(all), TrustSignature: "", Source: "", SourceURL: ""},
			}
		}
	}
	return keys, nil
}

// ReloadCache queries the GitHub API and reloads the local moduleCache of module versions.
// Should be called at least once after initialisation and probably on regular
// intervals afterward to keep moduleCache up-to-date.
func (s *GitHubStore) ReloadCache(ctx context.Context) error {
	repos, err := s.searchModuleRepositories(ctx)
	if err != nil {
		return err
	}

	fresh := make(map[string][]*core.ModuleVersion)

	for _, repo := range repos {
		owner, name, err := getOwnerRepoName(repo)
		if err != nil {
			return err
		}

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
	// reduces write lock duration by not modifying the moduleCache directly
	// on each iteration.
	s.moduleMut.Lock()
	s.moduleCache = fresh
	s.moduleMut.Unlock()

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

func (s *GitHubStore) listAllRepoReleases(ctx context.Context, owner, repo string) ([]*github.RepositoryRelease, error) {
	var allReleases []*github.RepositoryRelease

	opts := &github.ListOptions{
		PerPage: 100,
	}
	for {
		releases, resp, err := s.client.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			return allReleases, err
		}
		allReleases = append(allReleases, releases...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allReleases, nil
}
func (s *GitHubStore) searchModuleRepositories(ctx context.Context) ([]*github.Repository, error) {
	var filters []string

	if s.ownerFilter != "" {
		filters = append(filters, fmt.Sprintf(`org:"%s"`, s.ownerFilter))
	}
	if s.topicFilter != "" {
		filters = append(filters, fmt.Sprintf(`topic:"%s"`, s.topicFilter))
	}
	return s.searchRepositories(ctx, filters)
}

func (s *GitHubStore) searchProviderRepositories(ctx context.Context) ([]*github.Repository, error) {
	var filters []string

	if s.ownerFilter != "" {
		filters = append(filters, fmt.Sprintf(`org:"%s"`, s.providerOwnerFilter))
	}
	if s.topicFilter != "" {
		filters = append(filters, fmt.Sprintf(`topic:"%s"`, s.providerTopicFilter))
	}
	return s.searchRepositories(ctx, filters)
}

// searchRepositories fetches all repositories matching the configured filters.
// When an error is returned, the repositories fetched up until the point of error
// is also returned.
func (s *GitHubStore) searchRepositories(ctx context.Context, filters []string) ([]*github.Repository, error) {
	var (
		allRepos []*github.Repository
	)

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

func cacheKey(s ...string) string {
	return strings.Join(s, "/")
}

// extractOsArch inputs the filename of the Github release asset.
// Function uses regex to extract operating system and architecture about the binary inside the release
// Example input: terraform-provider-test_1.0.3_darwin_arm64.zip, output would be darwin as OS and arm64 as arch
func extractOsArch(name string) (core.Platform, bool) {
	matches := releaseRegex.FindStringSubmatch(name)

	if len(matches) >= 3 {
		osType := matches[1]
		arch := matches[2]
		return core.Platform{
			OS:   osType,
			Arch: arch,
		}, true
	}

	return core.Platform{}, false
}

// Provider Protocol version should be set in the terraform-registry-manifest.json file in the root of the repo.
// This file should be included in the release. If not present, default is 5.0 according to Terraform docs.
// https://developer.hashicorp.com/terraform/registry/providers/publishing
func (s *GitHubStore) getProviderProtocols(ctx context.Context, owner string, repo string, assets []*github.ReleaseAsset) ([]string, error) {
	providerProtocols := []string{"5.0"}

	for _, asset := range assets {
		if strings.Contains(asset.GetName(), "manifest.json") {

			responseBody, _, err := s.client.Repositories.DownloadReleaseAsset(ctx, owner, repo, asset.GetID(), http.DefaultClient)
			if err != nil {
				return nil, fmt.Errorf("unable to get manifest: %s", err)
			}

			manifest := &core.ProviderManifest{}
			err = json.NewDecoder(responseBody).Decode(manifest)
			responseBody.Close()
			if err != nil {
				return nil, fmt.Errorf("unable to decode manifest: %s", err)
			}

			providerProtocols = manifest.Metadata.ProtocolVersions
			break
		}
	}
	return providerProtocols, nil
}

// A file named SHA256SUMS containing sums must be included in the release. Look for the file, and download it
func (s *GitHubStore) getSHA256Sums(ctx context.Context, owner string, repo string, assets []*github.ReleaseAsset) (map[string]string, string, string, error) {
	var (
		SHASums        map[string]string
		SHASumURL      string
		SHASumFileName string
	)

	for _, asset := range assets {
		if strings.Contains(asset.GetName(), "SHA256SUMS") && !strings.HasSuffix(asset.GetName(), ".sig") {
			responseBody, _, err := s.client.Repositories.DownloadReleaseAsset(ctx, owner, repo, asset.GetID(), http.DefaultClient)
			if err != nil {
				return nil, "", "", fmt.Errorf("unable to get SHA checksums: %s", err)
			}

			SHASums = parseSHASumsFile(responseBody)
			SHASumURL = asset.GetBrowserDownloadURL()
			SHASumFileName = asset.GetName()
			responseBody.Close()
			break
		}
	}

	return SHASums, SHASumURL, SHASumFileName, nil
}

// Splitting owner from FullName to avoid getting it from GetOwner().GetName(),
// as it seems to be empty, maybe due to missing OAuth permission scopes.
func getOwnerRepoName(repo *github.Repository) (string, string, error) {
	parts := strings.Split(repo.GetFullName(), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo.FullName is not in expected format 'owner/repo', is '%s'", repo.GetFullName())
	}

	owner := parts[0]
	name := parts[1]
	return owner, name, nil
}
