// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package fs // import \"github.com/terramate-io/terramate/fs\""
  description = "package fs // import \"github.com/terramate-io/terramate/fs\"\n\nPackage fs provides filesystem related functionality.\n\nfunc CopyAll(dstdir, srcdir string) error\nfunc CopyDir(destdir, srcdir string, filter CopyFilterFunc) error\nfunc ListTerramateDirs(dir string) ([]string, error)\nfunc ListTerramateFiles(dir string) (filenames []string, err error)\ntype CopyFilterFunc func(path string, entry os.DirEntry) bool"
  tags        = ["fs", "golang"]
  id          = "68556dbf-d1d3-47de-a840-f9792dd06fa0"
}
