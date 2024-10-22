// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package genhcl // import \"github.com/terramate-io/terramate/generate/genhcl\""
  description = "package genhcl // import \"github.com/terramate-io/terramate/generate/genhcl\"\n\nPackage genhcl implements generate_hcl code generation.\n\nconst HeaderMagic = \"TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT\" ...\nconst ErrParsing errors.Kind = \"parsing generate_hcl block\" ...\nfunc DefaultHeader() string\nfunc Header(comment CommentStyle) string\ntype CommentStyle int\n    const SlashComment CommentStyle = iota ...\n    func CommentStyleFromConfig(tree *config.Tree) CommentStyle\ntype HCL struct{ ... }\n    func Load(root *config.Root, st *config.Stack, evalctx *eval.Context, ...) ([]HCL, error)"
  tags        = ["generate", "genhcl", "golang"]
  id          = "140a4bdc-6dc0-4d89-9c0b-9c54256345db"
}
