// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

# You can define multiple organizations in this file.
#
# The `name` field of the organization MUST match the `global.testserver.orgs.<name>`.
# Each organization MUST have an unique `uuid`.
#
# The required fields are:
#   - name
#   - uuid
#
# Make sure you have your membership set in the `members` field.
# Check the `test-cloud-users.tm` file.

globals "testserver" {
  default_test_org = "terramate"
}

globals "testserver" {
  orgs = {
    (global.testserver.default_test_org) = {
      name         = global.testserver.default_test_org
      display_name = "Terramate"
      domain       = "terramate.io"
      status       = "active"
      uuid         = "deadbeef-dead-dead-dead-deaddeadbeef"

      members = [
        for name, membership in global.testserver.memberships.terramate : {
          user_uuid = tm_try(global.testserver.users[name].user_uuid, null)
          apikey    = tm_try(global.testserver.apikeys[name].value, null)
          role      = membership.role
          status    = membership.status
        }
      ]

      stacks = [
        global.testserver.stacks.sample,
      ]
    }
  }
}
