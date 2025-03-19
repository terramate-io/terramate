// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "version" {
  content = <<-EOT
package version // import "github.com/terramate-io/terramate/commands/version"

Package version provides the version command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-version.ignore"
}
