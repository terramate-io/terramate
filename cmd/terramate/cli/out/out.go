// Package out provides output functionality, including verboseness level
// and normal/error messages support.
package out

import (
	"fmt"
	"io"
)

// O represents an output instance
type O struct {
	verboseness int
	stdout      io.Writer
	stderr      io.Writer
}

// New creates a new output with the given verboseness.
// Any output messages with a verboseness bigger than the given
// verboseness will be ignored.
func New(verboseness int, stdout, stderr io.Writer) O {
	return O{
		verboseness: verboseness,
		stdout:      stdout,
		stderr:      stderr,
	}
}

// MsgStdOut Send a message to stdout Writer output with no or any verboseness
func (o O) MsgStdOut(format string, args ...interface{}) {
	o.writeV(0, o.stdout, format, args...)
}

// MsgStdOutV Send a message to stdout Writer output with verboseness level 1 or higher
func (o O) MsgStdOutV(format string, args ...interface{}) {
	o.writeV(1, o.stdout, format, args...)
}

// MsgStdOutVV Send a message to stdout Writer output with verboseness level 2 or higher
func (o O) MsgStdOutVV(format string, args ...interface{}) {
	o.writeV(2, o.stdout, format, args...)
}

// MsgStdOutVVV Send a message to stdout Writer output with verboseness level 3 or higher
func (o O) MsgStdOutVVV(format string, args ...interface{}) {
	o.writeV(3, o.stdout, format, args...)
}

// MsgStdOutV0 Send a message to stdout Writer output only with no verboseness
func (o O) MsgStdOutV0(format string, args ...interface{}) {
	o.write(0, o.stdout, format, args...)
}

// MsgStdOutV1 Send a message to stdout Writer output only with verboseness level 1
func (o O) MsgStdOutV1(format string, args ...interface{}) {
	o.write(1, o.stdout, format, args...)
}

// MsgStdOutV2 Send a message to stdout Writer output only with verboseness level 2
func (o O) MsgStdOutV2(format string, args ...interface{}) {
	o.write(2, o.stdout, format, args...)
}

// MsgStdOutV3 Send a message to stdout Writer output only with verboseness level 3
func (o O) MsgStdOutV3(format string, args ...interface{}) {
	o.write(3, o.stdout, format, args...)
}

// MsgStdErr Send a message to stderr Writer output with no or any verboseness
func (o O) MsgStdErr(format string, args ...interface{}) {
	o.writeV(0, o.stderr, format, args...)
}

// MsgStdErrV Send a message to stderr Writer output with verboseness level 1 or higher
func (o O) MsgStdErrV(format string, args ...interface{}) {
	o.writeV(1, o.stderr, format, args...)
}

// MsgStdErrVV Send a message to stderr Writer output with verboseness level 2 or higher
func (o O) MsgStdErrVV(format string, args ...interface{}) {
	o.writeV(2, o.stderr, format, args...)
}

// MsgStdErrVVV Send a message to stderr Writer output with verboseness level 3
func (o O) MsgStdErrVVV(format string, args ...interface{}) {
	o.writeV(3, o.stderr, format, args...)
}

// MsgStdErrV0 Send a message to stderr Writer output only with no verboseness
func (o O) MsgStdErrV0(format string, args ...interface{}) {
	o.write(0, o.stderr, format, args...)
}

// MsgStdErrV1 Send a message to stderr Writer output only with verboseness level 1
func (o O) MsgStdErrV1(format string, args ...interface{}) {
	o.write(1, o.stderr, format, args...)
}

// MsgStdErrV2 Send a message to stderr Writer output only with verboseness level 2
func (o O) MsgStdErrV2(format string, args ...interface{}) {
	o.write(2, o.stderr, format, args...)
}

// MsgStdErrV3 Send a message to stderr Writer output only with verboseness level 3
func (o O) MsgStdErrV3(format string, args ...interface{}) {
	o.write(3, o.stderr, format, args...)
}

func (o O) writeV(verboseness int, w io.Writer, format string, args ...interface{}) {
	if verboseness > o.verboseness {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}

func (o O) write(verboseness int, w io.Writer, format string, args ...interface{}) {
	if verboseness != o.verboseness {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}
