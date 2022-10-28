// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// Define different verboseness levels.
const (
	V int = iota
	VV
	VVV
	VVVV
)

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

// Msg send a message to the output with the given verboseness
func (o O) Msg(verboseness int, format string, args ...interface{}) {
	o.write(verboseness, o.stdout, format, args...)
}

// Err send a message to the error output with the given verboseness
func (o O) Err(verboseness int, format string, args ...interface{}) {
	o.write(verboseness, o.stderr, format, args...)
}

func (o O) write(verboseness int, w io.Writer, format string, args ...interface{}) {
	if verboseness > o.verboseness {
		return
	}
	fmt.Fprintln(w, fmt.Sprintf(format, args...))
}
