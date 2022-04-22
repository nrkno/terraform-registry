terraform {
  required_version = ">= 1.0"
}

module "foo" {
  source  = "localhost.localdomain:8080/nrkno/foo/foo"
  version = "2.1.1"
}
