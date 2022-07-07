// Copyright 2021 Mineiros GmbH
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
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStacksGlobals(t *testing.T) {
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
			want    runExpected
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
					add: globals(
						str("str", "string"),
						number("number", 777),
						boolean("bool", true),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						str("str", "string"),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						str("str", "string"),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/stack3",
					add: globals(
						str("str", "stack3-string"),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						str("str", "stacks-string"),
					),
				},
				{
					path: "/stack3",
					add: globals(
						str("str", "stack3-string"),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						expr("stack-dir-name", `tm_basename(tm_abspath("."))`),
					),
				},
			},
			want: runExpected{
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
					add: globals(
						str("str", "string"),
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
					add: globals(
						str("str", "string"),
					),
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			sandboxes := []sandbox.S{
				sandbox.New(t),
				sandbox.NoGit(t),
			}

			for _, s := range sandboxes {
				s.BuildTree(tcase.layout)

				for _, globalBlock := range tcase.globals {
					path := filepath.Join(s.RootDir(), globalBlock.path)
					test.AppendFile(t, path, "globals.tm",
						globalBlock.add.String())
				}

				test.WriteRootConfig(t, s.RootDir())

				ts := newCLI(t, project.AbsPath(s.RootDir(), tcase.wd))
				assertRunResult(t, ts.run("experimental", "globals"), tcase.want)
			}
		})
	}
}
