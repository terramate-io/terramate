// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package tui // import \"github.com/terramate-io/terramate/ui/tui\""
  description = "package tui // import \"github.com/terramate-io/terramate/ui/tui\"\n\nPackage tui provides the Terminal User Interface (TUI) of the CLI.\n\nconst ErrSetup errors.Kind = \"failed to setup Terramate\"\nconst KongContext contextStr = \"kong.context\"\nconst KongError contextStr = \"kong.error\"\nfunc ConfigureLogging(logLevel, logFmt, logdest string, stdout, stderr io.Writer) error\nfunc DefaultAfterConfigHandler(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error)\nfunc DefaultBeforeConfigHandler(ctx context.Context, c *CLI) (cmd commands.Executor, found bool, cont bool, err error)\ntype CLI struct{ ... }\n    func NewCLI(opts ...Option) (*CLI, error)\ntype FlagSpec struct{ ... }\ntype Handler func(ctx context.Context, c *CLI) (commands.Executor, bool, bool, error)\ntype Option func(*CLI) error\n    func WithCompactHelp(b bool) Option\n    func WithDescription(desc string) Option\n    func WithExpandSubcommandsInHelp() Option\n    func WithHelpPrinter(p kong.HelpPrinter) Option\n    func WithName(name string) Option\n    func WithSpecHandler[T any](a *T, beforeHandler, afterHandler Handler, checkers ...rootFlagCheckers[*T]) Option\n    func WithStderr(w io.Writer) Option\n    func WithStdin(r io.Reader) Option\n    func WithStdout(w io.Writer) Option\n    func WithVersion(v string) Option"
  tags        = ["golang", "tui", "ui"]
  id          = "74902d82-fd54-4c30-b905-cec7372c4639"
}
