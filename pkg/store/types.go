package store

import "context"

type ModuleVersion struct {
	Version   string
	GitRef    string // TODO: remove this?
	SourceURL string
}

type ModuleStore interface {
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*ModuleVersion, error)
	GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*ModuleVersion, error)
}
