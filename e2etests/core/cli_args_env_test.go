// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"regexp"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestBasicArgEnvFlags(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		`s:stacks/a`,
		`s:stacks/b`,
		`s:stacks/c`,
		`s:stacks/a/1`,
		`s:stacks/a/2`,
		`s:stacks/a/3`,

		// not a stack
		`d:stacks/d`,
	})

	t.Run("TM_ARG_CHDIR", func(t *testing.T) {
		t.Parallel()

		type testcase struct {
			env      []string
			expected string
		}

		for _, tc := range []testcase{
			{
				expected: nljoin(
					"stacks/a",
					"stacks/a/1",
					"stacks/a/2",
					"stacks/a/3",
					"stacks/b",
					"stacks/c",
				),
			},
			{
				env: []string{"TM_ARG_CHDIR=."},
				expected: nljoin(
					"stacks/a",
					"stacks/a/1",
					"stacks/a/2",
					"stacks/a/3",
					"stacks/b",
					"stacks/c",
				),
			},
			{
				env: []string{"TM_ARG_CHDIR=stacks"},
				expected: nljoin(
					"a",
					"a/1",
					"a/2",
					"a/3",
					"b",
					"c",
				),
			},
			{
				env: []string{"TM_ARG_CHDIR=stacks/a"},
				expected: nljoin(
					".",
					"1",
					"2",
					"3",
				),
			},
			{
				env: []string{"TM_ARG_CHDIR=" + filepath.Join(s.RootDir(), "stacks/a")},
				expected: nljoin(
					".",
					"1",
					"2",
					"3",
				),
			},
			{
				env: []string{"TM_ARG_CHDIR=stacks/d"},
			},
		} {
			tc := tc
			tmcli := NewCLI(t, "", tc.env...)
			tmcli.SetWorkingDir(s.RootDir())
			AssertRunResult(t,
				tmcli.Run("list"),
				RunExpected{
					Stdout: tc.expected,
				},
			)
		}
	})

	t.Run("TM_ARG_RUN_NO_RECURSIVE=true", func(t *testing.T) {
		t.Parallel()

		// boolean flags accepts "true" and "false"
		tmcli := NewCLI(t, s.RootDir(), "TM_ARG_RUN_NO_RECURSIVE=true")
		AssertRunResult(t,
			tmcli.Run("run", "--", HelperPath, "echo", "hello"),
			RunExpected{
				Status:      1,
				StderrRegex: regexp.QuoteMeta(`Error: --no-recursive provided but no stack found`),
			},
		)

		// boolean flags also accepts "0" and "1"
		tmcli = NewCLI(t, s.RootDir(), "TM_ARG_RUN_NO_RECURSIVE=1")
		AssertRunResult(t,
			tmcli.Run("run", "--", HelperPath, "echo", "hello"),
			RunExpected{
				Status:      1,
				StderrRegex: regexp.QuoteMeta(`Error: --no-recursive provided but no stack found`),
			},
		)
	})
}
