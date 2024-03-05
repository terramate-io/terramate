// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"

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
