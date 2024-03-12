/**
 * Copyright 2023 Terramate GmbH
 * SPDX-License-Identifier: MPL-2.0
 */

resource "local_file" "foo" {
  content  = "hello world"
  filename = "${path.module}/foo.bar"
}
