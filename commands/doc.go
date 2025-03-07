// Package commands define all Terramate commands.
// All commands must:
// - Be defined in its own package (eg.: ./commands/generate)
// - Define a `<cmd>.Spec` type declaring all variables that control the command behavior.
// - Define a `<cmd>.Spec.Execute(ctx)` implementing the [command.Executor] interface.
// - Never abort, panic or exit the process.
package commands
