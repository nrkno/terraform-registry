<!--
SPDX-FileCopyrightText: 2022 NRK

SPDX-License-Identifier: GPL-3.0-only
-->

# Terraform Registry

An implementation of a private Terraform registry.

**NOTE:** the APIs of this app is not currently considered stable and may
change without notice before hitting v1.0.

**NOTE:** please question and report issues you might have with this implementation.
There is surely a lot of room for improvement.

## Features

Supported Terraform protocols:
- [ ] login.v1
- [x] modules.v1
- [ ] providers.v1

Supported backends:
- `MemoryStore`: in-memory store that can be populated manually
  - Must be populated manually
- `GitHubStore`: queries the GitHub API for modules, version tags and SSH download URLs
  - A query for module `namespace/name/provider` will return repository `namespace/name`
  - `provider` part of module URLs is not implemented. Can be set to anything at all.
  - Works for a single specified org/user
  - No verification for the repo actually being a Terraform repo

## Running

Environment variables:
- `LISTEN_ADDR` HTTP(S) listen address (default: `:8080`)
- `AUTH_DISABLED` disable authentication (default: `false`)
- `AUTH_TOKEN_FILE` filename with newline-separated strings of valid tokens
- `GITHUB_TOKEN` auth token for the GitHub API
- `GITHUB_ORG_NAME` name of org or user to search for repositories in
- `TLS_ENABLED` enable TLS (default: `false`)
- `TLS_CERT_FILE` path to a TLS certificate
- `TLS_KEY_FILE` path to a TLS key file

Build and run

```
$ make build
$ ./terraform-registry
```

## Development

Terraform does not allow disabling TLS certificate verification when accessing
a registry. Unless you have a valid certificate (signed by a valid CA) for your
hostname you will have to patch and build Terraform from source to disable the
TLS certificate validation.

### Build Terraform

Clone the official repository for the version you are using locally

```
$ terraform version
Terraform v1.1.9
on linux_amd6
$ git clone https://github.com/hashicorp/terraform --ref=v1.1.9 --depth=1
```

Apply the patch


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

### Create a self-signed TLS certificate

A script and an OpenSSL config file to create a self-signed certificate is
included in [./_tools/openssl/](./_tools/openssl/).

Update *openssl.conf* with your desired `[alternate_names]` and run
*create_self_signed_cert.sh*.

<small>(source: <https://stackoverflow.com/a/46100856>)</small>

### Run registry locally

```
$ export LISTEN_ADDR=:8080 AUTH_DISABLED=true
$ export GIT_HUB_ORG_NAME=myorg GIT_HUB_TOKEN=mytoken
$ export TLS_ENABLED=true TLS_CERT_FILE=cert.crt TLS_KEY_FILE=cert.key
$ make run
```

Now use `localhost.localdomain` as the registry URL for your module sources
in Terraform

```terraform
module "foo" {
  source  = "localhost.localdomain:8080/myorg/my-terraform-module-repo/generic//my-module"
  version = "~> 2.0"
}
```

### Testing

```
$ make test
```

### Adding license information

This adds or updates licensing information of all relevant files in the respository
using [reuse](https://git.fsfe.org/reuse/tool#install). It is available in some package
managers and in The Python Package Index as `reuse` (`pip install reuse`).

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
