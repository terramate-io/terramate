// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestParallelFibonacci(t *testing.T) {
	t.Parallel()

	type testcase struct {
		Name      string
		FibN      int
		WantValue int
	}

	for _, tc := range []testcase{
		{
			Name:      "fib(7)",
			FibN:      7,
			WantValue: 13,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			layout := makeFibLayout(tc.FibN)

			s := sandbox.NoGit(t, true)
			s.BuildTree(layout)

			tmcli := NewCLI(t, s.RootDir())

			res := tmcli.Run("run", "--quiet", "--parallel=5", "--", HelperPath, "fibonacci")
			AssertRunResult(t, res, RunExpected{})

			b, err := os.ReadFile(s.RootDir() + fmt.Sprintf("/fib.%v/fib.txt", tc.FibN))
			assert.NoError(t, err)
			got, err := strconv.ParseInt(string(b), 10, 64)
			assert.NoError(t, err)

			assert.EqualInts(t, tc.WantValue, int(got))
		})
	}
}

func TestParallelBug1828Regression(t *testing.T) {
	// see: https://github.com/terramate-io/terramate/issues/1828

	s := sandbox.NoGit(t, true)
	layout := []string{}
	for i := 0; i < 10; i++ {
		layout = append(layout, fmt.Sprintf(`s:stack-%d`, i))
	}
	s.BuildTree(layout)
	tmcli := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		tmcli.Run("run", "--parallel=2", "--quiet", "--", "cat", "foo.txt"),
		RunExpected{
			Status:      1,
			StderrRegex: "No such file or directory",
		},
	)

	// dry-run check
	AssertRunResult(t,
		tmcli.Run("run", "--parallel=2", "--dry-run", "--", "cat", "foo.txt"),
		RunExpected{
			Status:      0,
			StderrRegex: regexp.QuoteMeta("terramate: (dry-run)"),
		},
	)
}

func TestParalleCmdNotFoundContinueOnError(t *testing.T) {
	s := sandbox.NoGit(t, true)
	layout := []string{}
	for i := 0; i < 10; i++ {
		layout = append(layout, fmt.Sprintf(`s:stack-%d`, i))
	}
	s.BuildTree(layout)
	tmcli := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		tmcli.Run("run", "--parallel=2", "--quiet", "--", "do-not-exist"),
		RunExpected{
			Status:      1,
			StderrRegex: "executable file not found in",
		},
	)

	// dry-run check
	AssertRunResult(t,
		tmcli.Run("run", "--parallel=2", "--dry-run", "--", "do-not-exist"),
		RunExpected{
			Status:      1,
			StderrRegex: regexp.QuoteMeta("terramate: (dry-run)"),
		},
	)
}

func makeFibLayout(n int) []string {
	mkStack := func(i int) string {
		if i == 0 {
			return `s:fib.0`
		} else if i == 1 {
			return `s:fib.1`
		} else {
			return fmt.Sprintf(`s:fib.%v:after=["../fib.%v", "../fib.%v"]`, i, i-1, i-2)
		}
	}

	layout := make([]string, n+1)
	for i := 0; i <= n; i++ {
		layout[i] = mkStack(i)
	}

	return layout
}
