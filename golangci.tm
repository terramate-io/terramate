// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "/.golangci.toml" {
  context = root

  lets {
    config = {
      version = "2"
      linters = {
        enable = [
          "revive",
          "misspell",
          "bodyclose",
        ]

        exclusions = {
          generated = "lax"
          paths = [
            "third_party$",
            "builtin$",
            "examples$",
          ]

          rules = [
            {
              path    = "(.+)_test\\.go"
              text    = "dot-imports:"
              linters = ["revive"]
            }
          ]
        }
      }

      formatters = {
        enable = [
          "gofmt",
        ]
        exclusions = {
          generated = "lax"
          paths = [
            "third_party$",
            "builtin$",
            "examples$",
          ]
        }
      }

      run = {
        timeout : "15m"
      }
    }
  }

  content = tm_tomlencode(let.config)
}
