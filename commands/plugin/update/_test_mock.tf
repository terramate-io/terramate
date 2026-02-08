// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "update" {
  content = <<-EOT
package update // import "github.com/terramate-io/terramate/commands/plugin/update"

Package update provides the plugin update command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-update.ignore"
}
