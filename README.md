# Terraform Registry

An implementation of a private Terraform registry.

**NOTE:** the APIs of this app is not currently considered stable and may
change without notice before hitting v1.0.

Currently supported Terraform protocols:
- [ ] login.v1
- [x] modules.v1
- [ ] providers.v1

Currently supported backends:
- `MemoryStore`: in-memory store that can be populated manually
- `GitHubStore`: queries the GitHub API for modules, version tags and SSH download URLs

## Running

Build and run

```
$ go build ./cmd/terraform-registry/
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

Create an OpenSSL configuration file to simplify creation of a self-signed cert.

```
$ tee > openssl.conf
[CA_default]
copy_extensions = copy

[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = req_distinguished_name
x509_extensions = v3_ca

[req_distinguished_name]
C = NO
ST = Oslo
L = Oslo
O = Internet Widgits Pty Ltd
OU = Example
emailAddress = someone@example.com
CN = example.com

[v3_ca]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
subjectAltName = @alternate_names

[alternate_names]
DNS.1 = localhost
DNS.2 = localhost.localdomain
```

Create a self-signed certificate and private key

```
$ openssl req -x509 -newkey rsa:4096 -sha256 -utf8 -days 365 -nodes -config openssl.cnf -keyout cert.key -out cert.crt
```

(source: https://stackoverflow.com/a/46100856/90674)

### Run registry locally

```
$ export LISTEN_ADDR=:8080 AUTH_DISABLED=true
$ export GIT_HUB_ORG_NAME=myorg GIT_HUB_TOKEN=mytoken
$ export TLS_ENABLED=true TLS_CERT_FILE=cert.crt TLS_KEY_FILE=cert.key
$ go run .
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
$ go test -v ./...
```

## License

See [./LICENSE](./LICENSE)
