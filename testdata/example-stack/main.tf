/**
 * Copyright 2023 Terramate GmbH
 * SPDX-License-Identifier: MPL-2.0
 */

resource "local_file" "foo" {
  content  = var.content
  filename = "${path.module}/foo.bar"
}

variable "content" {
  sensitive = true
}
