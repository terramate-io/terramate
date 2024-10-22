// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package eval // import \"github.com/terramate-io/terramate/hcl/eval\""
  description = "package eval // import \"github.com/terramate-io/terramate/hcl/eval\"\n\nPackage eval provides both full and partial evaluation of HCL.\n\nconst ErrPartial errors.Kind = \"partial evaluation failed\" ...\nconst ErrCannotExtendObject errors.Kind = \"cannot extend object\"\nconst ErrEval errors.Kind = \"eval expression\"\ntype Context struct{ ... }\n    func NewContext(funcs map[string]function.Function) *Context\n    func NewContextFrom(ctx *hhcl.EvalContext) *Context\ntype CtyValue struct{ ... }\n    func NewCtyValue(val cty.Value, origin Info) CtyValue\ntype Info struct{ ... }\ntype Object struct{ ... }\n    func NewObject(origin Info) *Object\ntype ObjectPath []string\ntype Value interface{ ... }\n    func NewValue(val cty.Value, origin Info) Value"
  tags        = ["eval", "golang", "hcl"]
  id          = "5d3518ee-f9fa-414c-a446-e2d134e43239"
}
