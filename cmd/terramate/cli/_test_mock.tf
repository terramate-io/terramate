// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "cli" {
  content = <<-EOT
package cli // import "github.com/terramate-io/terramate/cmd/terramate/cli"

Package cli provides cli specific functionality.

const ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch" ...
const ErrLoginRequired errors.Kind ...
const ProvisionerTerraform = "terraform" ...
const ErrRunFailed errors.Kind = "execution failed" ...
func Exec(version string, args []string, stdin io.Reader, stdout io.Writer, ...)
type UIMode int
    const HumanMode UIMode = iota ...
EOT

  filename = "${path.module}/mock-cli.ignore"
}
