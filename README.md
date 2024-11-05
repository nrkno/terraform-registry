<!--
SPDX-FileCopyrightText: 2022 - 2024 NRK

SPDX-License-Identifier: MIT
-->

# Terraform Registry

This is an implementation of the Terraform registry protocol used to host a
private Terraform registry. Supports modular stores (backends) for discovering
and exposing modules and providers.

**NOTE:** the APIs of this program are not currently considered stable and
might introduce breaking changes in minor versions before v1 major is reached.

Please question and report any issues you encounter with this implementation.
There is surely room for improvement. Raise an issue discussing your proposed
changes before submitting a PR. There is however no guarantee we will merge
incoming pull requests.

Third-party provider registries (like this program) are supported only in
Terraform CLI v0.13 and later.

## Features

- [ ] login.v1 ([issue](https://github.com/nrkno/terraform-registry/issues/20))
- [x] modules.v1
- [x] providers.v1

### Stores

| Store | modules.v1 | providers.v1 | Description |
|:---|:---:|:---:|:---|
| GitHubStore | ✅ | ✅ | Uses the GitHub API to discover module and/or provider repositories using repository topics. |
| MemoryStore | ✅ | ❌ | A dumb in-memory store used for internal unit testing. |
| S3Store     | ✅ | ❌ | Uses the S3 protocol to discover modules stored in a bucket. |

### Authentication

You can configure the registry to require client authentication for the
`/v1/*` and `/download/*` paths. Additionally, the different stores might
implement other authentication schemes and details.

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

These are the common configuration options. Stores might have specific options
you can read more about in the stores section.

#### Command line arguments

- `-access-log-disabled`: Disable HTTP access log (default: `false`)
- `-access-log-ignored-paths`: Ignore certain request paths from being logged (default: `""`)
- `-listen-addr`: HTTP server bind address (default: `:8080`)
- `-auth-disabled`: Disable HTTP bearer token authentication (default: `false`)
- `-auth-tokens-file`: JSON encoded file containing a map of auth token descriptions and tokens.
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

#### Environment variables

- `ASSET_DOWNLOAD_AUTH_SECRET`: secret used to sign JWTs protecting the `/download/provider/` routes.

### GitHub Store

This store uses GitHub as a backend. Terraform modules and providers are discovered
by [applying topics to your organisation's GitHub repositories][repository topics].
The store is configured by setting up search filters for the owner/org, and a topic
you want to use to expose repository releases in the registry.

The registry requires a GitHub token that has read access to all repositories in the
organisation.

[repository topics]: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics

#### Modules

A query for the module address `namespace/name/provider` will return the GitHub repository `namespace/name`.
The `provider` part of the module URL must always be set to `generic` since
this store implementation has no notion of the type of providers the modules
are designed for.

Upon loading the list of repositories, tags prefixed with `v` will have their prefix removed.
I.e., a repository tag `v1.2.3` will be made available in the registry as version `1.2.3`.

No verification is performed to check if the repo actually contains a Terraform module.
This is left for Terraform to determine itself.

The module source download URLs returned are using the [`git::ssh` prefix](https://developer.hashicorp.com/terraform/language/modules/sources#generic-git-repository),
meaning that the client requesting the module must have a local SSH key linked with their
GitHub user, and this user must have read access to the repository in question. In other words,
repository source access is still maintained and handled by GitHub.

#### Providers

A query for the provider address `namespace/name` will return the GitHub repository `namespace/name`.

Some simple verification steps are performed to help ensure that the repo contains a Terraform
provider. A GitHub Release in the repository must follow the same
[steps that HashiCorp requires when publishing a provider](https://developer.hashicorp.com/terraform/registry/providers/publishing)
to their public registry.

In addition, you must provide the public part of the GPG signing key as part the Github release.
This is done by adding the GPG key in PEM format to your repository, and then
extending the `extra_files` object of the `.goreleaser.yaml` from Hashicorp.

Releases that do not follow this format will be ignored for the lifetime of the registry process and
will not be attempted verified again.

Example:
```yaml
release:
  extra_files:
    - glob: 'terraform-registry-manifest.json'
      name_template: '{{ .ProjectName }}_{{ .Version }}_manifest.json'
    - glob: 'gpg-public-key.pem'
      name_template: '{{ .ProjectName }}_{{ .Version }}_gpg-public-key.pem'
```

#### Environment variables

- `GITHUB_TOKEN`: auth token for the GitHub API

#### Command line arguments

- `-store github`
- `-github-owner-filter`: Module discovery GitHub org/user repository filter
- `-github-topic-filter`: Module discovery GitHub topic repository filter
- `-github-providers-owner-filter`: Provider discovery GitHub org/user repository filter
- `-github-providers-topic-filter`: Provider discovery GitHub topic repository filter

### S3 Store

This store uses S3 as a backend. A query for the module address
`namespace/name/provider` will be used directly as an S3 bucket key.
Modules must therefore be stored under keys in the following format
`namespace/name/provider/v1.2.3/v1.2.3.zip`.

The module source download URLs returned are using the [`s3::https` prefix](https://developer.hashicorp.com/terraform/language/modules/sources#s3-bucket),
meaning that the client requesting the module must have local access to the S3 bucket.

The registry requires the `s3:ListBucket` permission to discover modules, and
the clients will require the `s3:GetObject` permission.

No verification is performed to check if the path actually contains a Terraform
module. This is left for Terraform to determine.

#### Command line arguments

- `-store s3`: Switch store to S3
- `-s3-region`: Region such as us-east-1
- `-s3-bucket`: S3 bucket name

## Development

See [HACKING.md](./HACKING.md).

## References

- <https://www.terraform.io/language/modules/sources>
- <https://www.terraform.io/internals/login-protocol>
- <https://www.terraform.io/internals/module-registry-protocol>
- <https://www.terraform.io/internals/provider-registry-protocol>

## License

This project and all its files are licensed under MIT, unless stated
otherwise with a different license header. See [./LICENSES](./LICENSES) for
the full license text of all used licenses.
