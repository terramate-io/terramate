// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "project" {
  content = <<-EOT
package project // import "github.com/terramate-io/terramate/project"

Package project defines concepts that are related to a Terramate project.

const MaxGlobalLabels = 256
func AbsPath(root, prjAbsPath string) string
func FriendlyFmtDir(root, wd, dir string) (string, bool)
type Path struct{ ... }
    func NewPath(p string) Path
    func PrjAbsPath(root, abspath string) Path
type Paths []Path
type Runtime map[string]cty.Value
EOT

  filename = "${path.module}/mock-project.ignore"
}
