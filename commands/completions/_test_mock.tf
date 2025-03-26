// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "completions" {
  content = <<-EOT
package completions // import "github.com/terramate-io/terramate/commands/completions"

Package completions provides the install-completions command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-completions.ignore"
}
