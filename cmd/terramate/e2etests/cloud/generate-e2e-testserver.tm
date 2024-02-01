// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "testdata/cloud.data.json" {
  stack_filter {
    project_paths = [
      "/cmd/terramate/e2etests/cloud"
    ]
  }

  lets {
    version = tm_trimspace(tm_file("${terramate.root.path.fs.absolute}/VERSION"))
    well_known = {
      required_version = "= ${tm_replace(let.version, "/-.*/", "")}"
    }
  }

  # default users of the e2e test server.
  lets {
    users = {
      batman = {
        display_name = "Batman"
        email        = "batman@terramate.io"
        job_title    = "Entrepreneur"
        user_uuid    = "deadbeef-dead-dead-dead-deaddeadbeef"
      }
    }
  }

  # default organizations of the e2e test server.
  lets {
    orgs = {
      terramate = {
        name         = "terramate"
        display_name = "Terramate"
        domain       = "terramate.io"
        status       = "active"
        uuid         = "deadbeef-dead-dead-dead-deaddeadbeef"

        members = [
          for username, membership in let.memberships.terramate : {
            user_uuid = let.users[username].user_uuid
            role      = membership.role
            status    = membership.status
          }
        ]

        stacks = []
      }
    }
  }

  # default memberships of the e2e test server.
  lets {
    memberships = {
      terramate = {
        batman = {
          role   = "member"
          status = "active"
        }
      }
    }
  }

  content = tm_jsonencode({
    orgs       = let.orgs,
    users      = let.users,
    well_known = let.well_known,
  })
}
