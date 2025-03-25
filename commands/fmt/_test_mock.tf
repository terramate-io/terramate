// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "fmt" {
  content = <<-EOT
package fmt // import "github.com/terramate-io/terramate/commands/fmt"

Package fmt provides the fmt command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-fmt.ignore"
}
