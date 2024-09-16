// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package info // import \"github.com/terramate-io/terramate/hcl/info\""
  description = "package info // import \"github.com/terramate-io/terramate/hcl/info\"\n\nPackage info provides informational types related to hcl.\n\ntype Pos struct{ ... }\n    func NewPos(p hcl.Pos) Pos\ntype Range struct{ ... }\n    func NewRange(rootdir string, r hcl.Range) Range\ntype Ranges []Range"
  tags        = ["golang", "hcl", "info"]
  id          = "ed8a764a-e3bb-4811-8677-ce125db06359"
}
