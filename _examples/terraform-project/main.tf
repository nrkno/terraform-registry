terraform {
  required_version = ">= 1.0"
}

module "foo" {
  source  = "localhost.localdomain:8080/terraform-aws-modules/terraform-aws-vpc/generic"
  version = "~> 3.13"
}
