// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

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

type ModuleOauth2 interface {
	ClientID() string
	Endpoint() string
	ValidCode(ctx context.Context, code, redirectUrl, codeVerifier string) error
}
