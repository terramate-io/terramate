// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "cloud.data.json" {
  lets {
    version = tm_trimspace(tm_file("${terramate.root.path.fs.absolute}/VERSION"))
    well_known = {
      required_version = "= ${tm_replace(let.version, "/-.*/", "")}"
    }
  }
  content = tm_jsonencode({
    orgs       = global.testserver.orgs,
    users      = global.testserver.users,
    well_known = let.well_known,
  })
}
