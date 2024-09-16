// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "info" {
  content = <<-EOT
package info // import "github.com/terramate-io/terramate/hcl/info"

Package info provides informational types related to hcl.

type Pos struct{ ... }
    func NewPos(p hcl.Pos) Pos
type Range struct{ ... }
    func NewRange(rootdir string, r hcl.Range) Range
type Ranges []Range
EOT

  filename = "${path.module}/mock-info.ignore"
}
