terraform {
  required_version = ">= 1.0"
}

module "foo" {
  source  = "localhost.localdomain:8080/stigok/plattform-terraform-repository-release-test/generic//akamai-property"
  version = "~> 2.0"
  #source = "git::ssh://git@github.com/stigok/plattform-terraform-repository-release-test.git//akamai-property"
}
