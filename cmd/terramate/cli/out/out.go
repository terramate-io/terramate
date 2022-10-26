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
	level  int
	stdout io.Writer
	stderr io.Writer
}

// New creates a new output with the given level.
// Any output messages with a level bigger than the given
// level will be ignored.
func New(level int, stdout, stderr io.Writer) O {
	return O{
		level:  level,
		stdout: stdout,
		stderr: stderr,
	}
}

// Msg send a message to the output with the given verboseness
func (o O) Msg(level int, format string, args ...interface{}) {
	if level > o.level {
		return
	}
	fmt.Fprintln(o.stdout, fmt.Sprintf(format, args...))
}
