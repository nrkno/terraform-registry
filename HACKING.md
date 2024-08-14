<!--
SPDX-FileCopyrightText: 2022 - 2024 NRK

SPDX-License-Identifier: GPL-3.0-only
-->

# Terraform Registry
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

#### Build and Run the Private Registry

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

### Adding license information

This adds or updates licensing information of all relevant files in the
respository using [reuse](https://git.fsfe.org/reuse/tool#install).
It is available in some package managers and in The Python Package Index
as `reuse` (`pip install reuse>=4.0.3`).

```
$ make reuse
```
