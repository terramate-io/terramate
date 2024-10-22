// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "empty"
  description = "empty"
  id          = "04437b1f-27a9-4eab-8e63-1df968b76f6b"
  before      = ["/testdata/example-stack", "/testdata/testserver"]
  tags        = ["interop", "tag.with.dots-and-dash_and_underscore"]
}
