package main

import "sync"

type ModuleStore struct {
	store map[string]Module
	mut   sync.RWMutex
}

func NewModuleStore() *ModuleStore {
	return &ModuleStore{
		store: make(map[string]Module),
	}
}

func (ms *ModuleStore) Get(key string) *Module {
	ms.mut.RLock()
	defer ms.mut.RUnlock()

	m, ok := ms.store[key]
	if !ok {
		return nil
	}
	return &m
}

func (ms *ModuleStore) Set(key string, m Module) {
	ms.mut.Lock()
	defer ms.mut.Unlock()
	ms.store[key] = m
}
