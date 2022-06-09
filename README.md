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
There is surely room for improvement.

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

## Running

Build and run

```
$ make build
$ ./terraform-registry
```

## Configuring

All registry store types have some core options in common in addition to their
store specific ones.

Command line arguments:
- `-listen-addr`: HTTP server bind address (default: `:8080`)
- `-auth-disabled`: Disable HTTP bearer token authentication (default: `false`)
- `-auth-tokens-file`: File containing tokens
  - If the file has a `.json` file extension its contents is expected to be in
    the following format:
    ```json
    {
      "description for some token": "some token",
      "description for some other token": "some other token"
    }
    ```
  - All other file extensions is expected to contain a newline separated list of
    plain text tokens.
- `-tls-enabled`: Whether to enable TLS termination (default: `false`)
- `-tls-cert-file`: Path to TLS certificate file
- `-tls-key-file`: Path to TLS certificate private key file

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
### Commit hygiene

This repository enforces conventional commit messages. Please install and make
use of the pre-commit git hooks using [`pre-commit`](https://pre-commit.com/).

```
$ pip install pre-commit
$ pre-commit install --install-hooks -t commit-msg
```

Moreover, commits that does more than what is described, does not do what is
described, or that contains multiple unrelated changes will be rejected, and
will require the committer to edit the commits and/or their messages.

### Testing the registry locally

Terraform does not allow disabling TLS certificate verification when accessing
a registry. Unless you have a valid certificate (signed by a valid CA) for your
hostname, you will have to patch and build Terraform from source to disable the
verification.

#### Build Terraform

Clone the official repository for the version you are using locally

```
$ terraform version
Terraform v1.1.9
on linux_amd6
$ git clone https://github.com/hashicorp/terraform --ref=v1.1.9 --depth=1
```

Apply the patch to the cloned repository


```diff
diff --git a/internal/httpclient/client.go b/internal/httpclient/client.go
index bb06beb..5f9e424 100644
--- a/internal/httpclient/client.go
+++ b/internal/httpclient/client.go
@@ -1,6 +1,7 @@
 package httpclient

 import (
+       "crypto/tls"
        "net/http"

        cleanhttp "github.com/hashicorp/go-cleanhttp"
@@ -10,9 +11,14 @@ import (
 // package that will also send a Terraform User-Agent string.
 func New() *http.Client {
        cli := cleanhttp.DefaultPooledClient()
+       transport := cli.Transport.(*http.Transport)
+       transport.TLSClientConfig = &tls.Config{
+               InsecureSkipVerify: true,
+       }
+
        cli.Transport = &userAgentRoundTripper{
                userAgent: UserAgentString(),
-               inner:     cli.Transport,
+               inner:     transport,
        }
        return cli
 }
```

Then build Terraform

```
$ go build .
```

#### Create a self-signed TLS certificate

A script and an OpenSSL config file to create a self-signed certificate is
included in [./_tools/openssl/](./_tools/openssl/).

Update *openssl.conf* with your desired `[alternate_names]` and run
*create_self_signed_cert.sh*.

<small>(source: <https://stackoverflow.com/a/46100856>)</small>

#### Build and Run

```
$ make build
$ export GITHUB_TOKEN=mytoken
$ ./terraform-registry -listen-addr=:8080 -auth-disabled=true \
    -tls-enabled=true -tls-cert-file=cert.crt -tls-key-file=cert.key \
    -store=github -github-owner-filter=myuserororg -github-topic-filter=terraform-module
```

Now use `localhost.localdomain:8080` as the registry URL for your module sources
in Terraform

```terraform
module "foo" {
  source  = "localhost.localdomain:8080/myuserororg/my-module-repo/generic//my-module"
  version = "~> 1.2.3"
}
```

#### Testing

```
$ make test
```

#### Adding license information

This adds or updates licensing information of all relevant files in the
respository using [reuse](https://git.fsfe.org/reuse/tool#install).
It is available in some package managers and in The Python Package Index
as `reuse` (`pip install reuse`).

```
$ make reuse
```

## References

- <https://www.terraform.io/language/modules/sources>
- <https://www.terraform.io/internals/login-protocol>
- <https://www.terraform.io/internals/module-registry-protocol>
- <https://www.terraform.io/internals/provider-registry-protocol>

## License

This project and all its files are licensed under GNU GPL v3 unless stated
otherwise with a different license header. See [./LICENSES](./LICENSES) for
the full license text of all used licenses.
