// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "extension" {
  content = <<-EOT
package extension // import "github.com/terramate-io/terramate/plugin/extension"

Package extension defines the in-process extension interfaces.

type BindingsSetupHandler func(c CLI, bindings *di.Bindings) error
type CLI interface{ ... }
type CLIInfo interface{ ... }
type CommandSelector func(ctx context.Context, c CLI, command string, flags any) (commands.Command, error)
type Extension interface{ ... }
type PostInitEngineHook func(ctx context.Context, c CLI) error
EOT

  filename = "${path.module}/mock-extension.ignore"
}
