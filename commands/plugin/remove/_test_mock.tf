// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "remove" {
  content = <<-EOT
package remove // import "github.com/terramate-io/terramate/commands/plugin/remove"

Package remove provides the plugin remove command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-remove.ignore"
}
