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

package e2etest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestVersionCheck(t *testing.T) {
	checkedCmds := []string{
		"metadata",
		"generate",
		"stacks list",
		"stacks init stack",
		"stacks globals",
		"plan run-order",
		"plan graph",
	}
	uncheckedCmds := []string{
		"--help",
		"version",
	}
	run := func(t *testing.T, cmd string) runResult {
		s := sandbox.New(t)
		s.BuildTree([]string{"s:stack"})
		root := s.RootEntry()
		root.CreateConfig(`terramate {
			required_version = "= 0.0.0"
		}`)
		cli := newCLI(t, s.RootDir())
		return cli.run(strings.Split(cmd, " ")...)
	}

	for _, checkedCmd := range checkedCmds {
		name := fmt.Sprintf("%s is checked", checkedCmd)
		t.Run(name, func(t *testing.T) {
			assertRunResult(t, run(t, checkedCmd), runExpected{
				Status:      1,
				StderrRegex: terramate.ErrVersion.Error(),
			})
		})
	}
	for _, uncheckedCmd := range uncheckedCmds {
		name := fmt.Sprintf("%s isnt checked", uncheckedCmd)
		t.Run(name, func(t *testing.T) {
			assertRunResult(t, run(t, uncheckedCmd), runExpected{
				Status:       0,
				IgnoreStdout: true,
			})
		})
	}
}
