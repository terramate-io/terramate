// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

# Define the users and the organizations memberships in this file.

# Users definition:
# Each user MUST have an unique `user_uuid`.
# Required fields:
#   - email
#   - user_uuid

globals "testserver" "users" "batman" {
  display_name = "Batman"
  email        = "batman@terramate.io"
  job_title    = "Entrepreneur"
  user_uuid    = "deadbeef-dead-dead-dead-deaddeadbeef"
}

# Membership definition:
# Each attribute name MUST match the `testserver.users.<name>`.
# The membership fields `role` and `status` are required.

globals "testserver" "memberships" "terramate" {
  batman = {
    role   = "member"
    status = "active"
  }
}
