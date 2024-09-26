// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

globals {
  # you can enable this only in the stack you are debugging the tests.
  enable_test_logging = false

  is_go_package = tm_anytrue([
    tm_contains(terramate.stack.tags, "golang"),
    tm_contains(terramate.stack.tags, "e2etests"),
  ])
  has_test_files = tm_length(tm_fileset(".", "*_test.go")) > 0
}

generate_file "loglevel_disable_test.go" {
  condition = !global.enable_test_logging && global.is_go_package && global.has_test_files
  lets {
    special_cases = {
      "ls" = "tmls"
      "/"  = "terramate"
    }
    is_main  = tm_can(tm_file("main.go"))
    basename = terramate.stack.path.basename
    pkgname  = let.is_main ? "main" : tm_try(let.special_cases[let.basename], let.basename)
  }
  content = <<-EOF
      // Copyright 2024 Terramate GmbH
      // SPDX-License-Identifier: MPL-2.0

      package ${let.pkgname}_test

      import "github.com/rs/zerolog"

      func init() {
      	zerolog.SetGlobalLevel(zerolog.Disabled)
      }
    EOF
}
