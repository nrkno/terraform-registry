<!--
SPDX-FileCopyrightText: 2022 NRK

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
- [ ] providers.v1

Supported backends:
- `MemoryStore`: a dumb in-memory store currently only used for testing
- [`GitHubStore`](#github-store): queries the GitHub API for modules, version tags and SSH download URLs

Authentication:
- Reads a set of tokens from a file and authenticates requests based on the
  request's `Authorization: Bearer <token>`
- Only `/v1/*` routes are protected

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
- `-env-json-files`: A list of comma separated filenames pointing to JSON files
  containing an object whose keys and values will be loaded as environment vars.
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

Additionally, depending on the selected store type, some options are described
in the next subsections.

### GitHub Store

This store uses GitHub as a backend. A query for the module address
`namespace/name/provider` will return the GitHub repository `namespace/name`.
The `provider` part of the module URL must always be set to `generic` since
this store implementation has no notion of the type of providers the modules
are designed for.

Version strings are matched with repository tags. Upon loading the list of
repositories, tags prefixed with `v` will have their prefix removed.
I.e. a repository tag `v1.2.3` will be made available as version `1.2.3`.

No verification is performed to check if the repo actually contains Terraform
modules. This is left for Terraform to determine.

Environment variables:
- `GITHUB_TOKEN`: auth token for the GitHub API

Command line arguments:
- `-github-owner-filter`: GitHub org/user repository filter
- `-github-topic-filter`: GitHub topic repository filter

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
