// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package hclutils // import \"github.com/terramate-io/terramate/test/hclutils\""
  description = "package hclutils // import \"github.com/terramate-io/terramate/test/hclutils\"\n\nPackage hclutils provides test utils related to hcl.\n\nfunc End(line, column, char int) hhcl.Pos\nfunc FixupFiledirOnErrorsFileRanges(dir string, errs []error)\nfunc Mkrange(fname string, start, end hhcl.Pos) hhcl.Range\nfunc Start(line, column, char int) hhcl.Pos"
  tags        = ["golang", "hclutils", "test"]
  id          = "a1229483-5d7c-4745-bfe9-55b2b0825852"
}
