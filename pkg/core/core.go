// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"context"
	"io/fs"
)

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
	// Stores that do not implement this should return `nil, nil, fmt.Errorf("not implemented")`
	GetModuleVersionSource(ctx context.Context, namespace, name, provider, version string) (*ModuleVersion, fs.File, error)
}
