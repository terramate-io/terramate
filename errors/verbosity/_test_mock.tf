// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "verbosity" {
  content = <<-EOT
package verbosity // import "github.com/terramate-io/terramate/errors/verbosity"

Package verbosity defines the common Terramate error verbosity levels.

const V0 int = iota ...
EOT

  filename = "${path.module}/mock-verbosity.ignore"
}
