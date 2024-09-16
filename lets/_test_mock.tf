// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "lets" {
  content = <<-EOT
package lets // import "github.com/terramate-io/terramate/lets"

Package lets provides parsing and evaluation of lets blocks.

const ErrEval errors.Kind = "lets eval" ...
func Load(letblock *ast.MergedBlock, ctx *eval.Context) error
type Expr struct{ ... }
type Exprs map[string]Expr
type Map map[string]Value
type Value struct{ ... }
EOT

  filename = "${path.module}/mock-lets.ignore"
}
