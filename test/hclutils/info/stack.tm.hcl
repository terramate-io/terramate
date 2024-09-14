// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package info // import \"github.com/terramate-io/terramate/test/hclutils/info\""
  description = "package info // import \"github.com/terramate-io/terramate/test/hclutils/info\"\n\nPackage info provides functions useful to create types like info.Range\n\nfunc FixRange(rootdir string, old info.Range) info.Range\nfunc FixRangesOnConfig(dir string, cfg hcl.Config)\nfunc Range(fname string, start, end hhcl.Pos) info.Range"
  tags        = ["golang", "hclutils", "info", "test"]
  id          = "d997cacf-1b94-4315-a860-2ad654150d04"
}
