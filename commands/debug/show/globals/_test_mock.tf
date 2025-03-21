// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "globals" {
  content = <<-EOT
package globals // import "github.com/terramate-io/terramate/commands/debug/show/globals"

Package globals provides the debug globals command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-globals.ignore"
}
