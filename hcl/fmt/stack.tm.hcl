// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package fmt // import \"github.com/terramate-io/terramate/hcl/fmt\""
  description = "package fmt // import \"github.com/terramate-io/terramate/hcl/fmt\"\n\nPackage fmt contains functions for formatting hcl config.\n\nconst ErrHCLSyntax errors.Kind = \"HCL syntax error\"\nconst ErrReadFile errors.Kind = \"failed to read file\"\nfunc Format(src, filename string) (string, error)\nfunc FormatAttributes(attrs map[string]cty.Value) string\nfunc FormatMultiline(src, filename string) (string, error)\ntype FormatResult struct{ ... }\n    func FormatFiles(basedir string, files []string) ([]FormatResult, error)\n    func FormatTree(dir string) ([]FormatResult, error)"
  tags        = ["fmt", "golang", "hcl"]
  id          = "4c9c3934-72bb-4520-90c9-89de1dff254b"
}
