// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "globals" {
  content = <<-EOT
package globals // import "github.com/terramate-io/terramate/globals"

Package globals provides functions for loading globals.

const ErrEval errors.Kind = "global eval" ...
type EvalError struct{ ... }
type EvalReport struct{ ... }
    func ForDir(root *config.Root, cfgdir project.Path, ctx *eval.Context) EvalReport
    func ForStack(root *config.Root, stack *config.Stack) EvalReport
    func NewEvalReport() EvalReport
type Expr struct{ ... }
type ExprSet struct{ ... }
type GlobalPathKey struct{ ... }
    func NewGlobalAttrPath(basepath []string, name string) GlobalPathKey
    func NewGlobalExtendPath(path []string) GlobalPathKey
type HierarchicalExprs map[project.Path]*ExprSet
    func LoadExprs(tree *config.Tree) (HierarchicalExprs, error)
EOT

  filename = "${path.module}/mock-globals.ignore"
}
