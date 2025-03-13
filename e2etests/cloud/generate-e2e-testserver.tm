// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "testdata/cloud.data.json" {
  inherit = false
  lets {
    well_known = {
      required_version = "> 0.4.3"
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
    default_test_org = "terramate"
    orgs = {
      (let.default_test_org) = {
        name         = let.default_test_org
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

  lets {
    github_get_pull_request_response = tm_try(tm_jsondecode(tm_file("testdata/github/get_pull_request_response.json")), null)
    github_get_commit_response       = tm_try(tm_jsondecode(tm_file("testdata/github/get_commit_response.json")), null)
  }

  content = tm_jsonencode({
    default_test_org = let.default_test_org
    orgs             = let.orgs,
    users            = let.users,
    well_known       = let.well_known,
    github = {
      get_pull_request_response = let.github_get_pull_request_response,
      get_commit_response       = let.github_get_commit_response,
    }
  })
}
