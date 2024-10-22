// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "eval" {
  content = <<-EOT
package eval // import "github.com/terramate-io/terramate/hcl/eval"

Package eval provides both full and partial evaluation of HCL.

const ErrPartial errors.Kind = "partial evaluation failed" ...
const ErrCannotExtendObject errors.Kind = "cannot extend object"
const ErrEval errors.Kind = "eval expression"
type Context struct{ ... }
    func NewContext(funcs map[string]function.Function) *Context
    func NewContextFrom(ctx *hhcl.EvalContext) *Context
type CtyValue struct{ ... }
    func NewCtyValue(val cty.Value, origin Info) CtyValue
type Info struct{ ... }
type Object struct{ ... }
    func NewObject(origin Info) *Object
type ObjectPath []string
type Value interface{ ... }
    func NewValue(val cty.Value, origin Info) Value
EOT

  filename = "${path.module}/mock-eval.ignore"
}
