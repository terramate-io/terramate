// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "/.golangci.toml" {
  context = root

  lets {
    config = {
      linters = {
        enable = [
          "revive",
          "misspell",
          "gofmt",
          "bodyclose",
        ]
      }
      issues = {
        "exclude-rules" = [
          {
            path    = "(.+)_test\\.go"
            text = "dot-imports:"
            linters = ["revive"]
          }
        ]
        exclude-use-default = false
      }

      run = {
        timeout : "15m"
      }
    }
  }

  content = tm_tomlencode(let.config)
}
