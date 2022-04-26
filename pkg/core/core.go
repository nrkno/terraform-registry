package core

import "context"

type ModuleVersion struct {
	// Version is a SemVer version string that specifies the version for a module.
	Version string
	// SourceURL specifies the download URL where Terraform can get the module source.
	// https://www.terraform.io/language/modules/sources
	SourceURL string
}

// ModuleStore is the store implementation interface for building custom stores.
type ModuleStore interface {
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*ModuleVersion, error)
	GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*ModuleVersion, error)
}
