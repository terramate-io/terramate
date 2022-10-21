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

// newOutput creates a new output with the given level.
// Any output messages with a level bigger than the given
// level will be ignored.
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
