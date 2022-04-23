package main

import "fmt"

type Module struct {
	Namespace string
	Name      string
	System    string
	// Versions is a map where the key is a version string and value s the git ref
	Versions map[string]string
}

func (m *Module) String() string {
	return fmt.Sprintf("%s/%s/%s", m.Namespace, m.Name, m.System)
}

func (m *Module) HasVersion(version string) bool {
	for _, v := range m.Versions {
		if v == version {
			return true
		}
	}
	return false
}
