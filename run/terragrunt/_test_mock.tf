// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "terragrunt" {
  content = <<-EOT
package terragrunt // import "github.com/terramate-io/terramate/run/terragrunt"

Package terragrunt provides utilities for executing terragrunt commands with
configurable arguments.

type Runner struct{ ... }
    func NewRunner(env []string, workingDir string) *Runner
EOT

  filename = "${path.module}/mock-terragrunt.ignore"
}
