// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/nrkno/terraform-registry/pkg/core"
)

// TODO: move to core
// ModuleAddressPattern matches a module address like `namespace/name/provider`
var ModuleAddressPattern = regexp.MustCompile(`(?P<namespace>[^/\.]+)/(?P<name>[^/\.]+)/generic`)

// TODO: move to core
// SemVerPattern matches a SemVer version number like `1.2.3-beta1`
var SemVerPattern = regexp.MustCompile(`\d+.\d+.\d+\w*`)

// Store is an in-memory store implementation without a backend.
// Should not be instantiated directly. Use `NewStore` instead.
type Store struct {
	root  string
	store map[string][]*core.ModuleVersion
}

func NewStore(root string) (*Store, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("failed to stat root dir: %w", err)
	}

	return &Store{
		root:  root,
		store: make(map[string][]*core.ModuleVersion),
	}, nil
}

// Get returns a pointer to an item by key, or `nil` if it's not found.
func (s *Store) Get(key string) ([]*core.ModuleVersion, error) {
	match := ModuleAddressPattern.FindStringSubmatch(key)
	if match == nil {
		return nil, fmt.Errorf("invalid key: '%s' does not match pattern '%s'", key, ModuleAddressPattern)
	}

	namespace := match[ModuleAddressPattern.SubexpIndex("namespace")]
	name := match[ModuleAddressPattern.SubexpIndex("name")]

	repo, err := git.PlainOpen(path.Join(s.root, namespace, name))
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var versions []*core.ModuleVersion

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		// Only care about annotated tags
		// https://pkg.go.dev/github.com/go-git/go-git/v5#Repository.Tags
		tag, err := repo.TagObject(ref.Hash())
		if err != nil {
			return nil
		}

		// Only care about valid tags
		if !SemVerPattern.MatchString(tag.Name) {
			return nil
		}

		versions = append(versions, &core.ModuleVersion{
			Version:   tag.Name,
			SourceURL: "/download/" + namespace + "/" + name + "/" + tag.Name, // TODO: local downloading must be handled
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over repository tags: %w", err)
	}

	return versions, nil
}

// ListModuleVersions returns a list of module versions.
func (s *Store) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	return s.Get(key)
}

// GetModuleVersion returns single module version.
func (s *Store) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version '%s' not found for module '%s'", version, key)
}
