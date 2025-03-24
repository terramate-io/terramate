// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "generate" {
  content = <<-EOT
package generate // import "github.com/terramate-io/terramate/commands/generate"

Package generate provides the generate command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-generate.ignore"
}
