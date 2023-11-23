// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "/cmd/terramate/e2etests/cloud/testdata/cloud.data.json" {
  context = root

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
    orgs  = let.orgs,
    users = let.users,
  })
}
