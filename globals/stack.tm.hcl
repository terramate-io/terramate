// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package globals // import \"github.com/terramate-io/terramate/globals\""
  description = "package globals // import \"github.com/terramate-io/terramate/globals\"\n\nPackage globals provides functions for loading globals.\n\nconst ErrEval errors.Kind = \"global eval\" ...\ntype EvalError struct{ ... }\ntype EvalReport struct{ ... }\n    func ForDir(root *config.Root, cfgdir project.Path, ctx *eval.Context) EvalReport\n    func ForStack(root *config.Root, stack *config.Stack) EvalReport\n    func NewEvalReport() EvalReport\ntype Expr struct{ ... }\ntype ExprSet struct{ ... }\ntype GlobalPathKey struct{ ... }\n    func NewGlobalAttrPath(basepath []string, name string) GlobalPathKey\n    func NewGlobalExtendPath(path []string) GlobalPathKey\ntype HierarchicalExprs map[project.Path]*ExprSet\n    func LoadExprs(tree *config.Tree) (HierarchicalExprs, error)"
  tags        = ["globals", "golang"]
  id          = "4a34f564-d4a0-47e0-9dd7-75360f7e7af2"
}
