// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestImportsGlob(t *testing.T) {
	testcase := func(t *testing.T, pattern string, want runExpected) {
		s := sandbox.New(t)
		s.BuildTree([]string{
			`s:.`,
			`f:imports/DIR_1/file1.tm:globals {
				A = 1
			}`,
			`f:imports/DIR_2/file2.tm:globals {
				B = 2
			}`,
			`f:imports/DIR_3/file3.tm:globals {
				C = 3
			}`,
		})

		test.WriteFile(t, s.RootDir(),
			"imports.tm.hcl",
			fmt.Sprintf(`
		import {
			source = "%s"
		}
	`, pattern))

		tmcli := newCLI(t, s.RootDir())
		assertRunResult(t,
			tmcli.run("experimental", "globals"),
			want,
		)
	}

	allImportedResult := runExpected{
		Stdout: `
stack "/":
	A = 1
	B = 2
	C = 3
`,
	}

	t.Run("wildcard", func(t *testing.T) {
		testcase(t, "/imports/*/*.tm", allImportedResult)
	})
	t.Run("any pattern", func(t *testing.T) {
		testcase(t, "/imports/*/file[0-9].tm", allImportedResult)
	})

	t.Run("directory is not allowed", func(t *testing.T) {
		testcase(t, "/imports/*", runExpected{
			Status:      1,
			StderrRegex: `import directory is not allowed`,
		})
	})

	t.Run("double start is not allowed", func(t *testing.T) {
		testcase(t, "/**/*.tm", runExpected{
			Status:      1,
			StderrRegex: `returned no matches`,
		})
	})
}
