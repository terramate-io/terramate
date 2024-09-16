// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package modvendor // import \"github.com/terramate-io/terramate/modvendor\""
  description = "package modvendor // import \"github.com/terramate-io/terramate/modvendor\"\n\nPackage modvendor provides basic functions and types to support Terraform module\nvendoring.\n\nfunc AbsVendorDir(rootdir string, vendorDir project.Path, modsrc tf.Source) string\nfunc SourceDir(path string, rootdir string, vendordir project.Path) string\nfunc TargetDir(vendorDir project.Path, modsrc tf.Source) project.Path"
  tags        = ["golang", "modvendor"]
  id          = "2ca96d4e-828c-4f90-b67e-203f363c63a7"
}
