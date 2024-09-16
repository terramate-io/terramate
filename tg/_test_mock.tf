// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tg" {
  content = <<-EOT
package tg // import "github.com/terramate-io/terramate/tg"

Package tg implements functions to deal with Terragrunt files.

const ErrParsing errors.Kind = "parsing Terragrunt file"
type Module struct{ ... }
type Modules []*Module
    func ScanModules(rootdir string, dir project.Path, trackDependencies bool) (Modules, error)
EOT

  filename = "${path.module}/mock-tg.ignore"
}
