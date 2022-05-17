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

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestFormatRecursivelly(t *testing.T) {
	const unformattedHCL = `
globals {
name = "name"
		description = "desc"
	test = true
	}
	`
	formattedHCL, err := hcl.Format(unformattedHCL, "")
	assert.NoError(t, err)

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())

	t.Run("checking succeeds when there is no Terramate files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{})
	})

	t.Run("formatting succeeds when there is no Terramate files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt"), runExpected{})
	})

	sprintf := fmt.Sprintf
	s.BuildTree([]string{
		sprintf("f:globals.tm:%s", unformattedHCL),
		sprintf("f:another-stacks/globals.tm.hcl:%s", unformattedHCL),
		sprintf("f:another-stacks/stack-1/globals.tm.hcl:%s", unformattedHCL),
		sprintf("f:another-stacks/stack-2/globals.tm.hcl:%s", unformattedHCL),
		sprintf("f:stacks/globals.tm:%s", unformattedHCL),
		sprintf("f:stacks/stack-1/globals.tm:%s", unformattedHCL),
		sprintf("f:stacks/stack-2/globals.tm:%s", unformattedHCL),
	})

	wantedFiles := []string{
		"globals.tm",
		"another-stacks/globals.tm.hcl",
		"another-stacks/stack-1/globals.tm.hcl",
		"another-stacks/stack-2/globals.tm.hcl",
		"stacks/globals.tm",
		"stacks/stack-1/globals.tm",
		"stacks/stack-2/globals.tm",
	}
	wantedFilesStr := strings.Join(wantedFiles, "\n") + "\n"

	assertWantedFilesContents := func(t *testing.T, want string) {
		t.Helper()

		for _, file := range wantedFiles {
			got := s.RootEntry().ReadFile(file)
			assert.EqualStrings(t, want, string(got))
		}
	}

	t.Run("checking fails with unformatted files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{
			Status: 1,
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, unformattedHCL)
	})

	t.Run("update unformatted files in place", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt"), runExpected{
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, formattedHCL)
	})

	t.Run("checking succeeds when all files are formatted", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{})
		assertWantedFilesContents(t, formattedHCL)
	})

	t.Run("formatting succeeds when all files are formatted", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt"), runExpected{})
		assertWantedFilesContents(t, formattedHCL)
	})
}
