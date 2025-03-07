package exit

type Status int

// Standard exit codes of Terramate
const (
	OK Status = iota
	Failed
	Changed

	// this can be extended by external commands.
)
