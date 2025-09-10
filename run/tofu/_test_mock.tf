// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tofu" {
  content = <<-EOT
package tofu // import "github.com/terramate-io/terramate/run/tofu"

Package tofu provides utilities for executing OpenTofu commands with
configurable arguments.

type Runner struct{ ... }
    func NewRunner(env []string, workingDir string) *Runner
EOT

  filename = "${path.module}/mock-tofu.ignore"
}
