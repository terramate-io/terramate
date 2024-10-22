// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package project // import \"github.com/terramate-io/terramate/project\""
  description = "package project // import \"github.com/terramate-io/terramate/project\"\n\nPackage project defines concepts that are related to a Terramate project.\n\nconst MaxGlobalLabels = 256\nfunc AbsPath(root, prjAbsPath string) string\nfunc FriendlyFmtDir(root, wd, dir string) (string, bool)\ntype Path struct{ ... }\n    func NewPath(p string) Path\n    func PrjAbsPath(root, abspath string) Path\ntype Paths []Path\ntype Runtime map[string]cty.Value"
  tags        = ["golang", "project"]
  id          = "7c65ebb6-5d66-49f8-a38c-0c39d51075f2"
}
