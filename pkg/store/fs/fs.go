// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package fs

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/nrkno/terraform-registry/pkg/core"
)

// FSStore is a local file system store implementation. Modules are expected to
// be stored in the following directory hierarchy:
// `<rootPath>/<namespace>/<name>/<system>/<version>/<version>.zip`
// Should not be instantiated directly. Use `NewFSStore` instead.
type FSStore struct {
	fs fs.FS
}

func NewFSStore(rootPath string) *FSStore {
	return &FSStore{
		fs: os.DirFS(rootPath),
	}
}

// Get returns a pointer to an item by key, or `nil` if it's not found.
func (s *FSStore) Get(key string) []*core.ModuleVersion {
	entries, err := fs.ReadDir(s.fs, key)
	if err != nil {
		return nil
	}

	var mv []*core.ModuleVersion
	for _, e := range entries {
		if e.IsDir() {
			mv = append(mv, &core.ModuleVersion{Version: e.Name(), SourceURL: ""})
		}
	}

	if len(mv) == 0 {
		return nil
	}

	return mv
}

// ListModuleVersions returns a list of module versions.
func (s *FSStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions := s.Get(key)
	if versions == nil {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	return versions, nil
}

// GetModuleVersion returns single module version.
func (s *FSStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions := s.Get(key)
	if versions == nil {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version '%s' not found for module '%s'", version, key)
}
