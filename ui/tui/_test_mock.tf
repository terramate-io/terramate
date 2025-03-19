// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "tui" {
  content = <<-EOT
package tui // import "github.com/terramate-io/terramate/ui/tui"

Package tui provides the Terminal User Interface (TUI) of the CLI.

const ErrSetup errors.Kind = "failed to setup Terramate"
const KongContext contextStr = "kong.context"
const KongError contextStr = "kong.error"
func ConfigureLogging(logLevel, logFmt, logdest string, stdout, stderr io.Writer) error
func DefaultAfterConfigHandler(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error)
func DefaultBeforeConfigHandler(ctx context.Context, c *CLI) (cmd commands.Executor, found bool, cont bool, err error)
type CLI struct{ ... }
    func NewCLI(opts ...Option) (*CLI, error)
type FlagSpec struct{ ... }
type Handler func(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error)
type Option func(*CLI) error
    func WithCompactHelp(b bool) Option
    func WithDescription(desc string) Option
    func WithExpandSubcommandsInHelp() Option
    func WithHelpPrinter(p kong.HelpPrinter) Option
    func WithName(name string) Option
    func WithSpecHandler[T any](a *T, beforeHandler, afterHandler Handler, checkers ...rootFlagCheckers[*T]) Option
    func WithStderr(w io.Writer) Option
    func WithStdin(r io.Reader) Option
    func WithStdout(w io.Writer) Option
    func WithVersion(v string) Option
EOT

  filename = "${path.module}/mock-tui.ignore"
}
