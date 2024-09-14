// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package tf // import \"github.com/terramate-io/terramate/tf\""
  description = "package tf // import \"github.com/terramate-io/terramate/tf\"\n\nPackage tf provides parsing and abstractions specific to Terraform.\n\nconst ErrUnsupportedModSrc errors.Kind = \"unsupported module source\" ...\nconst ErrHCLSyntax errors.Kind = \"HCL syntax error\"\nfunc IsStack(path string) (bool, error)\ntype Module struct{ ... }\n    func ParseModules(path string) ([]Module, error)\ntype Source struct{ ... }\n    func ParseSource(modsource string) (Source, error)"
  tags        = ["golang", "tf"]
  id          = "c5a8d204-0193-4beb-a0e5-28a0fbe798e3"
}
