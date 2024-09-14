// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package lets // import \"github.com/terramate-io/terramate/lets\""
  description = "package lets // import \"github.com/terramate-io/terramate/lets\"\n\nPackage lets provides parsing and evaluation of lets blocks.\n\nconst ErrEval errors.Kind = \"lets eval\" ...\nfunc Load(letblock *ast.MergedBlock, ctx *eval.Context) error\ntype Expr struct{ ... }\ntype Exprs map[string]Expr\ntype Map map[string]Value\ntype Value struct{ ... }"
  tags        = ["golang", "lets"]
  id          = "a9c2b782-c3ee-4930-ace1-e20ca045df86"
}
