package cli

import (
	"fmt"
	"io"
)

type output struct {
	level  int
	stdout io.Writer
	stderr io.Writer
}

func newOutput(level int, stdout, stderr io.Writer) output {
	return output{
		level:  level,
		stdout: stdout,
		stderr: stderr,
	}
}

func (o output) msg(level int, format string, args ...interface{}) {
	if level > o.level {
		return
	}
	fmt.Fprintln(o.stdout, fmt.Sprintf(format, args...))
}
