// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

# Define the apikeys of the testserver.

globals "testserver" "apikeys" "some_key" {
  value = "tmco_mykey_checksum"
}

globals "testserver" "memberships" "terramate" {
  some_key = {
    role   = "member"
    status = "active"
  }
}
