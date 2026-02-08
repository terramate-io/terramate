// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "execgrpc" {
  content = <<-EOT
package execgrpc // import "github.com/terramate-io/terramate/commands/plugin/execgrpc"

Package execgrpc executes plugin commands over gRPC.

type Spec struct{ ... }
EOT

  filename = "${path.module}/mock-execgrpc.ignore"
}
