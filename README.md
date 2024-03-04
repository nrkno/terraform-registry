<!--
SPDX-FileCopyrightText: 2022 NRK
SPDX-FileCopyrightText: 2023 NRK

SPDX-License-Identifier: GPL-3.0-only
-->

# Terraform Registry

This is an implementation of the Terraform registry protocol used to host a
private Terraform registry.

**NOTE:** the APIs of this program and its library are not currently considered
stable and may change at any time before v1.0 is reached.

Please question and report any issues you encounter with this implementation.
There is surely room for improvement. Raise an issue discussing your proposed
changes before submitting a PR. There is no guarantee we will merge incoming
pull requests.

Third-party provider registries are supported only in Terraform CLI v0.13 and
later. Prior versions do not support this protocol.

## Features

Supported Terraform protocols:
- [ ] login.v1
- [x] modules.v1
- [x] providers.v1 - only implemented in the GithubStore

Supported backends:
- `MemoryStore`: a dumb in-memory store currently only used for testing
- [`GitHubStore`](#github-store): queries the GitHub API for modules, providers, version tags and SSH download URLs
- [`S3Store`](#S3-store): queries a S3 bucket for modules, version tags and HTTPS download URLs

Authentication:
- Reads a set of tokens from a file and authenticates requests based on the
  request's `Authorization: Bearer <token>`
- `/v1/*` routes are protected
- `/download/*` routes are protected

## Running
### Native

```
$ make build
$ ./terraform-registry -h
```

### Docker

```
$ docker build -t terraform-registry .
$ docker run terraform-registry
```

## Configuring

All registry store types have some core options in common in addition to their
store specific ones.

Command line arguments:
- `-access-log-disabled`: Disable HTTP access log (default: `false`)
- `-access-log-ignored-paths`: Ignore certain request paths from being logged (default: `""`)
- `-listen-addr`: HTTP server bind address (default: `:8080`)
- `-auth-disabled`: Disable HTTP bearer token authentication (default: `false`)
- `-auth-tokens-file`: JSON encoded file containing a map of auth token
  descriptions and tokens.
  ```json
  {
    "description for some token": "some token",
    "description for some other token": "some other token"
  }
  ```
- `-env-json-files`: Comma-separated list of paths to JSON encoded files
  containing a map of environment variable names and values to set.
  Converts the keys to uppercase and replaces all occurences of `-` (dash) with
  `_` (underscore).
  E.g. prefix filepaths with 'myprefix_:' to prefix all keys in the file with
  'MYPREFIX_' before they are set.
  - All variable names will be converted to uppercase, and `-` will become `_`.
  - If the filenames are prefixed with `myprefix_:`, the resulting environment
    variable names from the specific file will be prefixed with `MYPREFIX_`
    (e.g. `github_:/secret/github.json`).
  - If a variable name is unable to be converted to a valid format, a warning is
    logged, but the parsing continues without errors.
- `-tls-enabled`: Whether to enable TLS termination (default: `false`)
- `-tls-cert-file`: Path to TLS certificate file
- `-tls-key-file`: Path to TLS certificate private key file
- `-log-level`: Log level selection: `debug`, `info`, `warn`, `error` (default: `info`)
- `-log-format`: Log output format selection: `json`, `console` (default: `console`)
- `-version`: Print version info and exit

Additionally, depending on the selected store type, some options are described
in the next subsections.

### Environment variables:
- `ASSET_DOWNLOAD_AUTH_SECRET`: secret used to sign JWTs protecting the `/download/provider/` routes.

### GitHub Store
This store uses GitHub as a backend.

#### Modules
A query for the module address `namespace/name/provider` will return the GitHub repository `namespace/name`.
The `provider` part of the module URL must always be set to `generic` since
this store implementation has no notion of the type of providers the modules
are designed for.

Version strings are matched with [repository topics][]. Upon loading the list of
repositories, tags prefixed with `v` will have their prefix removed.
I.e., a repository tag `v1.2.3` will be made available as version `1.2.3`.

No verification is performed to check if the repo actually contains Terraform
modules. This is left for Terraform to determine.

#### Providers
A query for the provider address `namespace/name` will return the GitHub repository `namespace/name`.

Some verifications are performed to ensure that the repo contains what seems to be a Terraform 
provider. The releases on the repository must follow the same [steps](https://developer.hashicorp.com/terraform/registry/providers/publishing)
that HashiCorp requires when publishing a provider to their public registry.

In addition, you must provide the public part of the GPG signing key as part the Github release.
This is done by adding the GPG key in PEM format to your repository, and then 
extending the `extra_files` object of the `.goreleaser.yaml` from Hashicorp. 

Example:
```yaml
release:
  extra_files:
    - glob: 'terraform-registry-manifest.json'
      name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'
    - glob: 'gpg-public-key.pem'
      name_template: '{{ .ProjectName }}_{{ .Version }}_gpg-public-key.pem'
```

#### Environment variables:
- `GITHUB_TOKEN`: auth token for the GitHub API

#### Command line arguments:
- `-github-owner-filter`: Module discovery GitHub org/user repository filter
- `-github-topic-filter`: Module discovery GitHub topic repository filter
- `-github-providers-owner-filter`: Provider discovery GitHub org/user repository filter
- `-github-providers-topic-filter`: Provider discovery GitHub topic repository filter

### S3 Store

This store uses S3 as a backend. A query for the module address
`namespace/name/provider` will be translated directly to a S3 bucket key.
This request happens server side and either a list of modules will be returned
or a link to a s3 bucket for the terraform client to use.
Required permissions for the registry: `s3:ListBucket`
Requests to download are authenticated by S3 using credentials on the client side.
Required permissions for registry clients: `s3:GetObject`
[terraform S3 configuration]: (https://developer.hashicorp.com/terraform/language/modules/sources#s3-bucket)

Version strings are matched with [repository topics][]. Upon loading the list of
repositories, tags prefixed with `v` will have their prefix removed.
Storage of modules in S3 must match `namespace/name/provider/v1.2.3/v1.2.3.zip`
I.e., a repository tag `v1.2.3` will be made available as version `1.2.3`.

No verification is performed to check if the repo actually contains Terraform
modules. This is left for Terraform to determine.

#### Command line arguments:
- `-store s3`: Switch store to S3
- `-s3-region`: Region such as us-east-1
- `-s3-bucket`: S3 bucket name

[repository topics]: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics

## Development

See [HACKING.md](./HACKING.md).

## References

- <https://www.terraform.io/language/modules/sources>
- <https://www.terraform.io/internals/login-protocol>
- <https://www.terraform.io/internals/module-registry-protocol>
- <https://www.terraform.io/internals/provider-registry-protocol>

## License

This project and all its files are licensed under GNU GPL v3 unless stated
otherwise with a different license header. See [./LICENSES](./LICENSES) for
the full license text of all used licenses.
