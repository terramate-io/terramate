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

	tfversion "github.com/hashicorp/go-version"
	"github.com/madlambda/spells/assert"
	tm "github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/versions"
)

func TestVersionCheck(t *testing.T) {
	t.Parallel()

	checkedCmds := []string{
		"experimental metadata",
		"experimental globals",
		"experimental run-order",
		"experimental run-graph",
		"generate",
		"list",
		fmt.Sprintf("run cat %s", stack.DefaultFilename),
	}
	uncheckedCmds := []string{
		"--help",
		"--version",
		"version",
	}

	run := func(t *testing.T, cmd string, version string) runResult {
		s := sandbox.New(t)
		s.BuildTree([]string{"s:stack"})
		root := s.RootEntry()
		root.CreateConfig(fmt.Sprintf(`terramate {
			required_version = "= %s"
		}`, version))

		// required because `terramate run` requires a clean repo.
		git := s.Git()
		git.CommitAll("everything")

		cli := newCLI(t, s.RootDir())
		return cli.run(strings.Split(cmd, " ")...)
	}

	const (
		invalidVersion = "0.0.0"
	)

	for _, checkedCmd := range checkedCmds {
		name := fmt.Sprintf("%s is checked", checkedCmd)
		t.Run(name, func(t *testing.T) {
			assertRunResult(t, run(t, checkedCmd, invalidVersion), runExpected{
				Status:      1,
				StderrRegex: string(versions.ErrCheck),
			})
		})
	}
	for _, uncheckedCmd := range uncheckedCmds {
		name := fmt.Sprintf("%s isnt checked", uncheckedCmd)
		t.Run(name, func(t *testing.T) {
			assertRunResult(t, run(t, uncheckedCmd, invalidVersion), runExpected{
				Status:       0,
				IgnoreStdout: true,
			})
		})
	}

	cmds := append(checkedCmds, uncheckedCmds...)
	for _, cmd := range cmds {
		name := fmt.Sprintf("%s works with valid version", cmd)
		t.Run(name, func(t *testing.T) {
			assertRunResult(t, run(t, cmd, tm.Version()), runExpected{
				Status:       0,
				IgnoreStdout: true,
			})
		})
	}
}

func TestProvidesCorrectVersion(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())
	want := tm.Version() + "\n"

	assertRunResult(t, cli.run("version"), runExpected{
		Status: 0,
		Stdout: want,
	})
	assertRunResult(t, cli.run("--version"), runExpected{
		Status: 0,
		Stdout: want,
	})
}

func TestTerramateHasValidSemver(t *testing.T) {
	t.Parallel()

	_, err := tfversion.NewSemver(tm.Version())
	assert.NoError(t, err, "terramate VERSION file has invalid version")
}
