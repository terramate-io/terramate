// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "eval" {
  content = <<-EOT
package eval // import "github.com/terramate-io/terramate/commands/experimental/eval"

Package eval provides the "experimental eval" and "experimental partial-eval"
commands.

type GetConfigValueSpec struct{ ... }
type PartialSpec struct{ ... }
type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-eval.ignore"
}
