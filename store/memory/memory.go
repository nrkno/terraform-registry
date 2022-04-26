package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/nrkno/terraform-registry/store"
)

type MemoryStore struct {
	store map[string][]*store.ModuleVersion
	mut   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		store: make(map[string][]*store.ModuleVersion),
	}
}

func (s *MemoryStore) Get(key string) []*store.ModuleVersion {
	s.mut.RLock()
	defer s.mut.RUnlock()

	m, ok := s.store[key]
	if !ok {
		return nil
	}
	return m
}

func (s *MemoryStore) Set(key string, m []*store.ModuleVersion) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.store[key] = m
}

func (s *MemoryStore) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*store.ModuleVersion, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, name, provider)
	versions := s.Get(key)
	if versions == nil {
		return nil, fmt.Errorf("module '%s' not found", key)
	}

	return versions, nil
}

func (s *MemoryStore) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*store.ModuleVersion, error) {
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
