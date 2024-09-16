// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package genfile // import \"github.com/terramate-io/terramate/generate/genfile\""
  description = "package genfile // import \"github.com/terramate-io/terramate/generate/genfile\"\n\nPackage genfile implements generate_file code generation.\n\nconst ErrInvalidContentType errors.Kind = \"invalid content type\" ...\nconst StackContext = \"stack\" ...\ntype File struct{ ... }\n    func Eval(block hcl.GenFileBlock, cfg *config.Tree, evalctx *eval.Context) (file File, skip bool, err error)\n    func Load(root *config.Root, st *config.Stack, parentctx *eval.Context, ...) ([]File, error)"
  tags        = ["generate", "genfile", "golang"]
  id          = "f9ecbd56-b4b3-4410-806f-bc412c5e3c36"
}
