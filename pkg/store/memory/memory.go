package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/nrkno/terraform-registry/pkg/core"
)

type MemoryStore struct {
	store map[string][]*core.ModuleVersion
	mut   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string][]*core.ModuleVersion),
	}
}

func (s *MemoryStore) Get(key string) []*core.ModuleVersion {
	s.mut.RLock()
	defer s.mut.RUnlock()

	m, ok := s.store[key]
	if !ok {
		return nil
	}
	return m
}

func (s *MemoryStore) Set(key string, m []*core.ModuleVersion) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.store[key] = m
}

func (s *MemoryStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions := s.Get(key)
	if versions == nil {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	return versions, nil
}

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
