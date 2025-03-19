// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "trigger" {
  content = <<-EOT
package trigger // import "github.com/terramate-io/terramate/commands/trigger"

Package trigger provides the trigger command.

type FilterSpec struct{ ... }
type PathSpec struct{ ... }
type StatusFilters struct{ ... }
EOT

  filename = "${path.module}/mock-trigger.ignore"
}
