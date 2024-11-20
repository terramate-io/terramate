// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package fs // import \"github.com/terramate-io/terramate/hcl/fmt/fs\""
  description = "package fs // import \"github.com/terramate-io/terramate/hcl/fmt/fs\"\n\nconst ErrReadFile errors.Kind = \"failed to read file\"\ntype FormatResult struct{ ... }\n    func FormatFiles(basedir string, files []string) ([]FormatResult, error)\n    func FormatTree(root *config.Root, dir project.Path) ([]FormatResult, error)"
  tags        = ["fmt", "fs", "golang", "hcl"]
  id          = "c8413056-70d3-4065-bbf9-839d12e12c69"
}
