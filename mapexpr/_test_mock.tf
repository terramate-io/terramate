// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "mapexpr" {
  content = <<-EOT
package mapexpr // import "github.com/terramate-io/terramate/mapexpr"

Package mapexpr implements the `map` block as an HCL expression type.

type Attributes struct{ ... }
type MapExpr struct{ ... }
    func NewMapExpr(block *ast.MergedBlock) (*MapExpr, error)
EOT

  filename = "${path.module}/mock-mapexpr.ignore"
}
