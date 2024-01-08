// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestListRunOrder(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		want   RunExpected
	}

	for _, tcase := range []testcase{
		{
			name: "stack1 after stack2",
			layout: []string{
				`s:stack1:after=["/stack2"]`,
				"s:stack2",
			},
			want: RunExpected{
				Stdout: "stack2\nstack1\n",
			},
		},
		{
			name: "cycle between stack1 and stack2",
			layout: []string{
				`s:stack1:after=["/stack2"]`,
				`s:stack2:after=["/stack1"]`,
			},
			want: RunExpected{
				Status: 1,
				StderrRegexes: []string{
					"Invalid stack configuration",
					"cycle detected",
				},
			},
		},
	} {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := NewCLI(t, s.RootDir())
			AssertRunResult(t, cli.Run("list", "--run-order"), tc.want)
		})
	}
}
