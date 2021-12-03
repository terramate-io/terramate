package cli_test

import (
	"testing"

	"github.com/mineiros-io/terrastack/test/sandbox"
)

func TestCLIList(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		want   runResult
	}

	for _, tc := range []testcase{
		{
			name: "no stack",
		},
		{
			name: "no stack, lots of dirs",
			layout: []string{
				"d:dir1/a/b/c",
				"d:dir2/a/b/c/x/y",
				"d:last/dir",
			},
		},
		{
			name:   "single stack",
			layout: []string{"s:stack"},
			want: runResult{
				Stdout: "stack\n",
			},
		},
		{
			name: "single stack down deep inside directories",
			layout: []string{
				"d:lots",
				"d:of",
				"d:directories",
				"d:lots/lots",
				"d:of/directories/without/any/stack",
				"d:but",
				"s:there/is/a/very/deep/hidden/stack/here",
				"d:more",
				"d:waste/directories",
			},
			want: runResult{
				Stdout: "there/is/a/very/deep/hidden/stack/here\n",
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: runResult{
				Stdout: "1\n2\n3\n",
			},
		},
		{
			name: "multiple stacks at multiple levels",
			layout: []string{
				"s:1",
				"s:2",
				"s:1/a",
				"s:1/b",
				"d:not-stack",
				"d:something/else/uninportant",
				"s:3",
				"s:3/x",
				"s:3/x/y",
				"s:3/x/y/z",
			},
			want: runResult{
				Stdout: `1
1/a
1/b
2
3
3/x
3/x/y
3/x/y/z
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.BaseDir())
			assertRunResult(t, cli.run("list"), tc.want)
		})
	}
}
