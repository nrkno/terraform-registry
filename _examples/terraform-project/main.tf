# SPDX-FileCopyrightText: 2022 NRK
# SPDX-FileCopyrightText: 2023 NRK
# SPDX-FileCopyrightText: 2024 NRK
#
# SPDX-License-Identifier: GPL-3.0-only

terraform {
  required_version = ">= 1.0"
}

module "foo" {
  source  = "localhost.localdomain:8080/terraform-aws-modules/terraform-aws-vpc/generic"
  version = "~> 3.13"
}
