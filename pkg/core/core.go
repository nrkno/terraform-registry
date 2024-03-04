// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"context"
	"io"
)

type ModuleVersion struct {
	// Version is a SemVer version string that specifies the version for a module.
	Version string
	// SourceURL specifies the download URL where Terraform can get the module source.
	// https://www.terraform.io/language/modules/sources
	SourceURL string
}

type ProviderVersions struct {
	Versions []ProviderVersion `json:"versions"`
}

type ProviderVersion struct {
	Version   string     `json:"version"`
	Protocols []string   `json:"protocols"`
	Platforms []Platform `json:"platforms"`
}

type Platform struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

type SigningKeys struct {
	GPGPublicKeys []GpgPublicKeys `json:"gpg_public_keys"`
}

type GpgPublicKeys struct {
	KeyID          string `json:"key_id"`
	ASCIIArmor     string `json:"ascii_armor"`
	TrustSignature string `json:"trust_signature"`
	Source         string `json:"source"`
	SourceURL      string `json:"source_url"`
}

type Provider struct {
	Protocols           []string    `json:"protocols"`
	OS                  string      `json:"os"`
	Arch                string      `json:"arch"`
	Filename            string      `json:"filename"`
	DownloadURL         string      `json:"download_url"`
	SHASumsURL          string      `json:"shasums_url"`
	SHASumsSignatureURL string      `json:"shasums_signature_url"`
	SHASum              string      `json:"shasum"`
	SigningKeys         SigningKeys `json:"signing_keys"`
}

func (p *Provider) Copy() *Provider {
	return &Provider{
		Protocols:           p.Protocols,
		OS:                  p.OS,
		Arch:                p.Arch,
		Filename:            p.Filename,
		DownloadURL:         p.DownloadURL,
		SHASumsURL:          p.SHASumsURL,
		SHASumsSignatureURL: p.SHASumsSignatureURL,
		SHASum:              p.SHASum,
		SigningKeys:         p.SigningKeys,
	}
}

type ProviderManifest struct {
	Version  int `json:"version"`
	Metadata struct {
		ProtocolVersions []string `json:"protocol_versions"`
	} `json:"metadata"`
}

// ModuleStore is the store implementation interface for building custom module stores.
type ModuleStore interface {
	ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*ModuleVersion, error)
	GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*ModuleVersion, error)
}

// ProviderStore is the store implementation interface for building custom provider stores
type ProviderStore interface {
	ListProviderVersions(ctx context.Context, namespace string, name string) (*ProviderVersions, error)
	GetProviderVersion(ctx context.Context, namespace string, name string, version string, os string, arch string) (*Provider, error)
	GetProviderAsset(ctx context.Context, namespace string, name string, tag string, asset string) (io.ReadCloser, error)
}
