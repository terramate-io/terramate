// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "terramate" {
  content = <<-EOT
package terramate // import "github.com/terramate-io/terramate"

Package terramate provides functions for managing terraform stacks. A stack is a
unit of independent runnable terraform modules.

func Version() string
EOT

  filename = "${path.module}/mock-terramate.ignore"
}
