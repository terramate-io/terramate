// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestStacksGlobals(t *testing.T) {
	t.Parallel()

	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name    string
			layout  []string
			wd      string
			globals []globalsBlock
			want    RunExpected
		}
	)

	tcases := []testcase{
		{
			name:   "no stacks no globals",
			layout: []string{},
		},
		{
			name:   "single stacks no globals",
			layout: []string{"s:stack"},
		},
		{
			name: "two stacks no globals",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name:   "single stack with a global, wd = root",
			layout: []string{"s:stack"},
			globals: []globalsBlock{
				{
					path: "/stack",
					add: Globals(
						Str("str", "string"),
						Number("number", 777),
						Bool("bool", true),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stack":
	bool   = true
	number = 777
	str    = "string"
`,
			},
		},
		{
			name: "two stacks only one has globals, wd = root",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("str", "string"),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stacks/stack-1":
	str = "string"
`,
			},
		},
		{
			name: "two stacks with same globals, wd = root",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			globals: []globalsBlock{
				{
					path: "/stacks",
					add: Globals(
						Str("str", "string"),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stacks/stack-1":
	str = "string"

stack "/stacks/stack-2":
	str = "string"
`,
			},
		},
		{
			name: "three stacks only two has globals, wd = stack3",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stack3",
			},
			wd: "/stack3",
			globals: []globalsBlock{
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("str", "string"),
					),
				},
				{
					path: "/stack3",
					add: Globals(
						Str("str", "stack3-string"),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stack3":
	str = "stack3-string"
`,
			},
		},
		{
			name: "three stacks with globals, wd = stacks",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stack3",
			},
			wd: "/stacks",
			globals: []globalsBlock{
				{
					path: "/stacks",
					add: Globals(
						Str("str", "stacks-string"),
					),
				},
				{
					path: "/stack3",
					add: Globals(
						Str("str", "stack3-string"),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stacks/stack-1":
	str = "stacks-string"

stack "/stacks/stack-2":
	str = "stacks-string"
`,
			},
		},
		{
			name: "abspath() do not use wd but stack dir as base, wd = stacks",
			layout: []string{
				"s:stacks/stack-name",
			},
			wd: "/stacks",
			globals: []globalsBlock{
				{
					path: "/stacks",
					add: Globals(
						Expr("stack-dir-name", `tm_basename(tm_abspath("."))`),
					),
				},
			},
			want: RunExpected{
				Stdout: `
stack "/stacks/stack-name":
	stack-dir-name = "stack-name"
`,
			},
		},
		{
			name: "two stacks with globals and one without, wd = stack3",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stack3",
			},
			wd: "/stack3",
			globals: []globalsBlock{
				{
					path: "/stacks",
					add: Globals(
						Str("str", "string"),
					),
				},
			},
		},
		{
			name: "two stacks with globals and wd = some-non-stack-dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"d:some-non-stack-dir",
			},
			wd: "/some-non-stack-dir",
			globals: []globalsBlock{
				{
					path: "/stacks",
					add: Globals(
						Str("str", "string"),
					),
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			sandboxes := []sandbox.S{
				sandbox.New(t),
				sandbox.NoGit(t, true),
			}

			for _, s := range sandboxes {
				s.BuildTree(tcase.layout)

				for _, globalBlock := range tcase.globals {
					path := filepath.Join(s.RootDir(), globalBlock.path)
					test.AppendFile(t, path, "globals.tm",
						globalBlock.add.String())
				}

				ts := NewCLI(t, project.AbsPath(s.RootDir(), tcase.wd))
				AssertRunResult(t, ts.Run("debug", "show", "globals"), tcase.want)
			}
		})
	}
}
