// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package tg // import \"github.com/terramate-io/terramate/tg\""
  description = "package tg // import \"github.com/terramate-io/terramate/tg\"\n\nPackage tg implements functions to deal with Terragrunt files.\n\nconst ErrParsing errors.Kind = \"parsing Terragrunt file\"\ntype Module struct{ ... }\ntype Modules []*Module\n    func ScanModules(rootdir string, dir project.Path, trackDependencies bool) (Modules, error)"
  tags        = ["golang", "tg"]
  id          = "84d2ad63-38a0-408f-9d11-3b95168d9484"
}
