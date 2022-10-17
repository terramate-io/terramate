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
	stdfmt "fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/fmt"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestFormatRecursively(t *testing.T) {
	t.Parallel()

	const unformattedHCL = `
globals {
name = "name"
		description = "desc"
	test = true
	}
	`
	formattedHCL, err := fmt.Format(unformattedHCL, "")
	assert.NoError(t, err)

	s := sandbox.New(t)
	cli := newCLI(t, s.RootDir())

	t.Run("checking succeeds when there is no Terramate files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{})
	})

	t.Run("formatting succeeds when there is no Terramate files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt"), runExpected{})
	})

	sprintf := stdfmt.Sprintf
	writeUnformattedFiles := func() {
		s.BuildTree([]string{
			sprintf("f:globals.tm:%s", unformattedHCL),
			sprintf("f:another-stacks/globals.tm.hcl:%s", unformattedHCL),
			sprintf("f:another-stacks/stack-1/globals.tm.hcl:%s", unformattedHCL),
			sprintf("f:another-stacks/stack-2/globals.tm.hcl:%s", unformattedHCL),
			sprintf("f:stacks/globals.tm:%s", unformattedHCL),
			sprintf("f:stacks/stack-1/globals.tm:%s", unformattedHCL),
			sprintf("f:stacks/stack-2/globals.tm:%s", unformattedHCL),
		})
	}

	writeUnformattedFiles()

	wantedFiles := []string{
		"globals.tm",
		"another-stacks/globals.tm.hcl",
		"another-stacks/stack-1/globals.tm.hcl",
		"another-stacks/stack-2/globals.tm.hcl",
		"stacks/globals.tm",
		"stacks/stack-1/globals.tm",
		"stacks/stack-2/globals.tm",
	}
	filesListOutput := func(files []string) string {
		portablewantedFiles := make([]string, len(files))
		for i, f := range files {
			portablewantedFiles[i] = filepath.FromSlash(f)
		}
		return strings.Join(portablewantedFiles, "\n") + "\n"
	}
	wantedFilesStr := filesListOutput(wantedFiles)

	assertFileContents := func(t *testing.T, path string, want string) {
		t.Helper()
		got := s.RootEntry().ReadFile(path)
		assert.EqualStrings(t, want, string(got))
	}

	assertWantedFilesContents := func(t *testing.T, want string) {
		t.Helper()

		for _, file := range wantedFiles {
			assertFileContents(t, file, want)
		}
	}

	t.Run("checking fails with unformatted files", func(t *testing.T) {
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{
			Status: 1,
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, unformattedHCL)
	})

	t.Run("checking fails with unformatted files on subdirs", func(t *testing.T) {
		subdir := filepath.Join(s.RootDir(), "another-stacks")
		cli := newCLI(t, subdir)
		assertRunResult(t, cli.run("fmt", "--check"), runExpected{
			Status: 1,
			Stdout: filesListOutput([]string{
				"globals.tm.hcl",
				"stack-1/globals.tm.hcl",
				"stack-2/globals.tm.hcl",
			}),
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

	t.Run("update unformatted files in subdirs", func(t *testing.T) {
		writeUnformattedFiles()

		anotherStacks := filepath.Join(s.RootDir(), "another-stacks")
		cli := newCLI(t, anotherStacks)
		assertRunResult(t, cli.run("fmt"), runExpected{
			Stdout: filesListOutput([]string{
				"globals.tm.hcl",
				"stack-1/globals.tm.hcl",
				"stack-2/globals.tm.hcl",
			}),
		})

		assertFileContents(t, "another-stacks/globals.tm.hcl", formattedHCL)
		assertFileContents(t, "another-stacks/stack-1/globals.tm.hcl", formattedHCL)
		assertFileContents(t, "another-stacks/stack-2/globals.tm.hcl", formattedHCL)

		assertFileContents(t, "globals.tm", unformattedHCL)
		assertFileContents(t, "stacks/globals.tm", unformattedHCL)
		assertFileContents(t, "stacks/stack-1/globals.tm", unformattedHCL)
		assertFileContents(t, "stacks/stack-2/globals.tm", unformattedHCL)

		stacks := filepath.Join(s.RootDir(), "stacks")
		cli = newCLI(t, stacks)
		assertRunResult(t, cli.run("fmt"), runExpected{
			Stdout: filesListOutput([]string{
				"globals.tm",
				"stack-1/globals.tm",
				"stack-2/globals.tm",
			}),
		})

		assertFileContents(t, "another-stacks/globals.tm.hcl", formattedHCL)
		assertFileContents(t, "another-stacks/stack-1/globals.tm.hcl", formattedHCL)
		assertFileContents(t, "another-stacks/stack-2/globals.tm.hcl", formattedHCL)
		assertFileContents(t, "stacks/globals.tm", formattedHCL)
		assertFileContents(t, "stacks/stack-1/globals.tm", formattedHCL)
		assertFileContents(t, "stacks/stack-2/globals.tm", formattedHCL)

		assertFileContents(t, "globals.tm", unformattedHCL)

		cli = newCLI(t, s.RootDir())
		assertRunResult(t, cli.run("fmt"), runExpected{
			Stdout: filesListOutput([]string{"globals.tm"}),
		})

		assertWantedFilesContents(t, formattedHCL)
	})
}
