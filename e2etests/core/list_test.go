// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"testing"

	"github.com/terramate-io/terramate/config"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

type testcase struct {
	name         string
	layout       []string
	filterTags   []string
	filterNoTags []string
	want         RunExpected
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin("there/is/a/very/deep/hidden/stack/here"),
			},
		},
		{
			name: "multiple stacks at same level",
			layout: []string{
				"s:1", "s:2", "s:3",
			},
			want: RunExpected{
				Stdout: nljoin("1", "2", "3"),
			},
		},
		{
			name: "stack inside other stack",
			layout: []string{
				"s:stack",
				"s:stack/child-stack",
			},
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin("dir/d", "dir/subdir/e"),
			},
		},
		{
			name:   "invalid stack.tags - starting with number - fails+",
			layout: []string{`s:stack:tags=["123abc"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with uppercase - fails",
			layout: []string{`s:stack:tags=["Abc"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with underscore - fails",
			layout: []string{`s:stack:tags=["_test"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - starting with dash - fails",
			layout: []string{`s:stack:tags=["-test"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - uppercase - fails",
			layout: []string{`s:stack:tags=["thisIsInvalid"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - dash in the end - fails",
			layout: []string{`s:stack:tags=["invalid-"]`},
			want: RunExpected{
				StderrRegex: string(config.ErrStackInvalidTag),
				Status:      1,
			},
		},
		{
			name:   "invalid stack.tags - underscore in the end - fails",
			layout: []string{`s:stack:tags=["invalid_"]`},
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: nljoin("stack-a", "stack-b"),
			},
		},
	}
}

func TestListStackWithDefinitionOnNonDefaultFilename(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{"d:stack"})
	stackDir := s.DirEntry("stack")
	stackDir.CreateFile("stack.tm", "stack {}")

	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.ListStacks(), RunExpected{Stdout: "stack\n"})
}

func TestListStackWithNoTerramateBlock(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})
	cli := NewCLI(t, s.RootDir())
	AssertRunResult(t, cli.ListStacks(), RunExpected{Stdout: "stack\n"})
}

func TestListLogsWarningIfConfigHasSchemaIssues(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name   string
		layout []string
		want   RunExpected
	}

	for _, tc := range []testcase{
		{
			name: "empty terramate block in child dirs do not warn",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate().String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
			},
		},
		{
			name: "empty terramate.config block in child dirs do not warn",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
			},
		},
		{
			name: "terramate.required_version in child dirs do WARN",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Str("required_version", "1.0.0"),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"required_version",
				},
			},
		},
		{
			name: "imported terramate.required_version in child dirs do WARN",
			layout: []string{
				"f:/modules/terramate.tm:" + Terramate(
					Str("required_version", "1.0.0"),
				).String(),
				"s:stack",
				`f:stack/import-block.tm:` + Import(
					Str("source", "/modules/terramate.tm"),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"required_version",
					"imported from directory",
				},
			},
		},
		{
			name: "terramate.config.git in child dirs do WARN",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(
						Block("git"),
					),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"block terramate\\.config\\.git can only be declared at the project root directory",
				},
			},
		},
		{
			name: "imported terramate.config.git in child dirs do WARN",
			layout: []string{
				"f:/modules/terramate.tm:" + Terramate(
					Config(
						Block("git"),
					),
				).String(),
				"s:stack",
				`f:stack/import-block.tm:` + Import(
					Str("source", "/modules/terramate.tm"),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"block terramate\\.config\\.git can only be declared at the project root directory",
					"imported from directory",
				},
			},
		},
		{
			name: "terramate.config.generate in child dirs do WARN",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(
						Block("generate"),
					),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"block terramate\\.config\\.generate can only be declared at the project root directory",
				},
			},
		},
		{
			name: "terramate.config.change_detection in child dirs do WARN",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(
						Block("change_detection"),
					),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"block terramate\\.config\\.change_detection can only be declared at the project root directory",
				},
			},
		},
		{
			name: "terramate.config.run.check_gen_code in child dirs do WARN",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(
						Block("run",
							Bool("check_gen_code", false),
						),
					),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
				StderrRegexes: []string{
					string(hcl.ErrTerramateSchema),
					"attribute terramate\\.config\\.run\\.check_gen_code can only be declared at the project root directory",
				},
			},
		},
		{
			name: "terramate.config.run.env block in child dirs do NOT warn",
			layout: []string{
				"s:stack",
				`f:stack/terramate.tm:` + Terramate(
					Config(
						Block("run",
							Block("env",
								Str("FOO", "BAR"),
							),
						),
					),
				).String(),
			},
			want: RunExpected{
				Stdout: nljoin("stack"),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			tmcli := NewCLI(t, s.RootDir())
			tmcli.LogLevel = "warn"
			AssertRunResult(t, tmcli.ListStacks(), tc.want)
		})
	}
}

func TestListNoSuchFile(t *testing.T) {
	t.Parallel()

	notExists := test.NonExistingDir(t)
	cli := NewCLI(t, notExists)

	AssertRunResult(t, cli.ListStacks(), RunExpected{
		Status:      1,
		StderrRegex: "changing working dir",
	})
}

func TestListRunOrderNotChangedStackIgnored(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName = "main.tf"
		mainTfContents = "# change is the eternal truth of the universe"
	)

	s := sandbox.New(t)

	// stack must run after stack2 but stack2 didn't change.
	s.BuildTree([]string{
		`s:stack:after=["/stack2"]`,
		"s:stack2",
	})

	stack := s.DirEntry("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")

	git := s.Git()
	git.CommitAll("first commit")
	git.Push("main")
	git.CheckoutNew("change-stack")

	stackMainTf.Write(mainTfContents)
	git.CommitAll("stack changed")

	cli := NewCLI(t, s.RootDir())

	AssertRunResult(t, cli.ListChangedStacks(), RunExpected{Stdout: nljoin(stack.RelPath())})
	AssertRunResult(t, cli.Run("list", "--changed", "--run-order"),
		RunExpected{
			Stdout: nljoin(stack.RelPath()),
		})
}
