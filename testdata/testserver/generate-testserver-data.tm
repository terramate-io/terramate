// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "cloud.data.json" {
  content = tm_jsonencode({
    orgs  = global.testserver.orgs,
    users = global.testserver.users,
  })
}
