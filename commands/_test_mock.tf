// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "commands" {
  content = <<-EOT
package commands // import "github.com/terramate-io/terramate/commands"

Package commands define all Terramate commands. All commands must: - Be defined
in its own package (eg.: ./commands/generate) - Define a `<cmd>.Spec` type
declaring all variables that control the command behavior. - Implement the
commands.Executor interface. - Never abort, panic or exit the process.

type Executor interface{ ... }
EOT

  filename = "${path.module}/mock-commands.ignore"
}
