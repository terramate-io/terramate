// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "terraform" {
  content = <<-EOT
package terraform // import "github.com/terramate-io/terramate/run/terraform"

Package terraform provides utilities for executing terraform commands with
configurable arguments.

type Runner struct{ ... }
    func NewRunner(env []string, workingDir string) *Runner
EOT

  filename = "${path.module}/mock-terraform.ignore"
}
