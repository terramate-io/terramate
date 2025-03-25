// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "exit" {
  content = <<-EOT
package exit // import "github.com/terramate-io/terramate/exit"

Package exit provides standard exit codes for Terramate.

type Status int
    const OK Status = iota ...
EOT

  filename = "${path.module}/mock-exit.ignore"
}
