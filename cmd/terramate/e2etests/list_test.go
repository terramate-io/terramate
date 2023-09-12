// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"testing"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

type testcase struct {
	name         string
	layout       []string
	filterTags   []string
	filterNoTags []string
	want         runExpected
}

func listTestcases() []testcase {
	return []testcase{
		{
			name: "no stack",
		},
		{
			name: "dot directories ignored",
			layout: []string{
				"f:.stack/stack.tm:stack {}",
			},
		},
		{
			name: "dot files ignored",
			layout: []string{
				"f:stack/.stack.tm:stack {}",
			},
		},
		{
			name: "dot directories ignored",
			layout: []string{
				"s:stack",
				"f:stack/.substack/stack.tm:stack {}",
			},
			want: runExpected{
				Stdout: "stack\n",
			},
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
			want: runExpected{
				Stdout: nljoin("stack"),
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
			want: runExpected{
				Stdout: nljoin("there/is/a/very/deep/hidden/stack/here"),
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: runExpected{
				Stdout: nljoin("1", "2", "3"),
			},
		},
		{
			name: "stack inside other stack",
			layout: []string{
				"s:stack",
				"s:stack/child-stack",
			},
			want: runExpected{
				Stdout: nljoin("stack", "stack/child-stack"),
			},
		},
		{
			name: "multiple levels of stacks inside stacks",
			layout: []string{
				"s:mineiros.io",
				"s:mineiros.io/departments",
				"s:mineiros.io/departments/engineering",
				"s:mineiros.io/departments/accounting",
				"s:mineiros.io/departments/engineering/terramate",
				"s:mineiros.io/departments/engineering/terraform-modules",
				"d:mineiros.io/departments/engineering/docs",
				"d:mineiros.io/departments/engineering/tests",
				"s:mineiros.io/departments/engineering/tests/e2e",
			},
			want: runExpected{
				Stdout: nljoin(
					"mineiros.io",
					"mineiros.io/departments",
					"mineiros.io/departments/accounting",
					"mineiros.io/departments/engineering",
					"mineiros.io/departments/engineering/terraform-modules",
					"mineiros.io/departments/engineering/terramate",
					"mineiros.io/departments/engineering/tests/e2e",
				),
			},
		},
		{
			name: "multiple stacks at multiple levels",
			layout: []string{
				"s:1",
				"s:2",
				"s:z/a",
				"s:x/b",
				"d:not-stack",
				"d:something/else/uninportant",
				"s:3/x/y/z",
			},
			want: runExpected{
				Stdout: nljoin("1", "2", "3/x/y/z", "x/b", "z/a"),
			},
		},
		{
			name: "multiple stacks filtered by same tag",
			layout: []string{
				`s:a:tags=["abc"]`,
				`s:b:tags=["abc"]`,
				`s:dir/c:tags=["abc"]`,
				`s:dir/d`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"abc"},
			want: runExpected{
				Stdout: nljoin("a", "b", "dir/c"),
			},
		},
		{
			name: "multiple stacks filtered by not having abc tag",
			layout: []string{
				`s:a:tags=["abc"]`,
				`s:b:tags=["abc"]`,
				`s:dir/c:tags=["abc"]`,
				`s:dir/d`,
				`s:dir/subdir/e`,
			},
			filterNoTags: []string{"abc"},
			want: runExpected{
				Stdout: nljoin("dir/d", "dir/subdir/e"),
			},
		},
		{
			name:   "invalid stack.tags - starting with number - fails+",
			layout: []string{`s:stack:tags=["123abc"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with uppercase - fails",
			layout: []string{`s:stack:tags=["Abc"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with underscore - fails",
			layout: []string{`s:stack:tags=["_test"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with dash - fails",
			layout: []string{`s:stack:tags=["-test"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - uppercase - fails",
			layout: []string{`s:stack:tags=["thisIsInvalid"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - dash in the end - fails",
			layout: []string{`s:stack:tags=["invalid-"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - underscore in the end - fails",
			layout: []string{`s:stack:tags=["invalid_"]`},
			want: runExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name: "stack.tags with digit in the end - works",
			layout: []string{
				`s:stack:tags=["a1", "b100", "c-1", "d_1"]`,
			},
			filterTags: []string{"a1"},
			want: runExpected{
				Stdout: nljoin("stack"),
			},
		},
		{
			name: "all stacks containing the tag `a`",
			layout: []string{
				`s:a:tags=["a", "b", "c", "d"]`,
				`s:b:tags=["a", "b"]`,
				`s:dir/c:tags=["a"]`,
				`s:dir/d`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"a"},
			want: runExpected{
				Stdout: nljoin("a", "b", "dir/c"),
			},
		},
		{
			name: "all stacks containing tags `a && b`",
			layout: []string{
				`s:a:tags=["a", "b", "c", "d"]`,
				`s:b:tags=["a", "b"]`,
				`s:dir/c:tags=["a"]`,
				`s:dir/d:tags=["c", "d"]`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"a:b"},
			want: runExpected{
				Stdout: nljoin("a", "b"),
			},
		},
		{
			name: "all stacks containing the tags `a && b && c`",
			layout: []string{
				`s:a:tags=["a", "b", "c", "d"]`,
				`s:b:tags=["a", "b"]`,
				`s:dir/c:tags=["a"]`,
				`s:dir/d:tags=["c", "d"]`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"a:b:c"},
			want: runExpected{
				Stdout: nljoin("a"),
			},
		},
		{
			name: "all stacks containing tag `a || b`",
			layout: []string{
				`s:a:tags=["a", "b", "c", "d"]`,
				`s:b:tags=["a", "b"]`,
				`s:dir/c:tags=["a"]`,
				`s:dir/d:tags=["c", "d"]`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"a,b"},
			want: runExpected{
				Stdout: nljoin("a", "b", "dir/c"),
			},
		},
		{
			name: "all stacks containing tags `a && b || c && d`",
			layout: []string{
				`s:a:tags=["a", "b", "c", "d"]`,
				`s:b:tags=["a", "b"]`,
				`s:dir/c:tags=["a"]`,
				`s:dir/d:tags=["c", "d"]`,
				`s:dir/subdir/e`,
			},
			filterTags: []string{"a:b,c:d"},
			want: runExpected{
				Stdout: nljoin("a", "b", "dir/d"),
			},
		},
		{
			name: "filters work with dash and underscore tags",
			layout: []string{
				`s:stack-a:tags=["terra-mate", "terra_mate"]`,
				`s:stack-b:tags=["terra_mate"]`,
				`s:no-tag-stack`,
			},
			filterTags: []string{"terra-mate,terra_mate"},
			want: runExpected{
				Stdout: nljoin("stack-a", "stack-b"),
			},
		},
		{
			name: "multiple --tags makes an OR clause with all flag values",
			layout: []string{
				`s:stack-a:tags=["terra-mate", "terra_mate"]`,
				`s:stack-b:tags=["terra_mate"]`,
				`s:no-tag-stack`,
			},
			filterTags: []string{
				"terra-mate",
				"terra_mate",
			},
			want: runExpected{
				Stdout: nljoin("stack-a", "stack-b"),
			},
		},
	}
}

func TestListStackWithDefinitionOnNonDefaultFilename(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{"d:stack"})
	stackDir := s.DirEntry("stack")
	stackDir.CreateFile("stack.tm", "stack {}")

	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListStackWithNoTerramateBlock(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})
	cli := newCLI(t, s.RootDir())
	assertRunResult(t, cli.listStacks(), runExpected{Stdout: "stack\n"})
}

func TestListLogsWarningIfConfigHasConflicts(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack",
		`f:stack/terramate.tm:terramate {}`,
	})

	tmcli := newCLI(t, s.RootDir())
	tmcli.loglevel = "warn"
	assertRunResult(t, tmcli.listStacks(), runExpected{
		Stdout:      "stack\n",
		StderrRegex: string(hcl.ErrUnexpectedTerramate),
	})
}

func TestListNoSuchFile(t *testing.T) {
	t.Parallel()

	notExists := test.NonExistingDir(t)
	cli := newCLI(t, notExists)

	assertRunResult(t, cli.listStacks(), runExpected{
		Status:      1,
		StderrRegex: "changing working dir",
	})
}
