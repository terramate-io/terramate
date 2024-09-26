// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// This file is required by the "preview" script.

generate_hcl "_test_mock.tf" {
  condition = global.is_go_package
  lets {
    name = tm_ternary(terramate.stack.path.basename == "/", "terramate", terramate.stack.path.basename)
  }
  content {
    tm_dynamic "resource" {
      labels = ["local_file", let.name]
      attributes = {
        content  = <<-EOF
          ${terramate.stack.description}
        EOF
        filename = "${path.module}/mock-${let.name}.ignore"
      }
    }
  }
}
