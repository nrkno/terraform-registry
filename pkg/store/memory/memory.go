// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/nrkno/terraform-registry/pkg/core"
)

// MemoryStore is an in-memory store implementation without a backend.
// Should not be instantiated directly. Use `NewMemoryStore` instead.
type MemoryStore struct {
	store map[string][]*core.ModuleVersion
	mut   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string][]*core.ModuleVersion),
	}
}

// Get returns a pointer to an item by key, or `nil` if it's not found.
func (s *MemoryStore) Get(key string) []*core.ModuleVersion {
	s.mut.RLock()
	defer s.mut.RUnlock()

	m, ok := s.store[key]
	if !ok {
		return nil
	}
	return m
}

// Set stores an item under the specified `key`.
func (s *MemoryStore) Set(key string, m []*core.ModuleVersion) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.store[key] = m
}

// ListModuleVersions returns a list of module versions.
func (s *MemoryStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions := s.Get(key)
	if versions == nil {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	return versions, nil
}

// GetModuleVersion returns single module version.
func (s *MemoryStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
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

func (s *MemoryStore) GetModuleVersionSource(ctx context.Context, namespace, name, system, version string) (*core.ModuleVersion, fs.File, error) {
	return nil, nil, fmt.Errorf("not implemented")
}
