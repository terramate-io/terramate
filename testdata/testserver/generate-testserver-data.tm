// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "cloud.data.json" {
  lets {
    well_known = {
      required_version = "> 0.4.3"
    }
  }
  content = tm_jsonencode({
    orgs       = global.testserver.orgs,
    users      = global.testserver.users,
    well_known = let.well_known,
  })
}
