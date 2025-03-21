// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "runtime_env" {
  content = <<-EOT
package runtimeenv // import "github.com/terramate-io/terramate/commands/debug/show/runtime_env"

Package runtimeenv provides the show-runtime-env command.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-runtime_env.ignore"
}
