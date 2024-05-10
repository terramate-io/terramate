// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "/cmd/terramate/e2etests/cloud/testdata/cloud.data.json" {
  context = root

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

  lets {
    github_get_pull_request_response = tm_jsondecode(tm_file("cmd/terramate/e2etests/cloud/testdata/github/get_pull_request_response.json"))
    github_get_commit_response = tm_jsondecode(tm_file("cmd/terramate/e2etests/cloud/testdata/github/get_commit_response.json"))
  }

  content = tm_jsonencode({
    orgs       = let.orgs,
    users      = let.users,
    well_known = let.well_known,
    github = {
      get_pull_request_response = let.github_get_pull_request_response,
      get_commit_response = let.github_get_commit_response,
    }
  })
}
