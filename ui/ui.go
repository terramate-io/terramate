package ui

// Executor is an interface to a Terramate UI (TUI/HTTP/etc)
type Executor interface {
	Exec(command string, insn any) (exitCode int, err error)
}
