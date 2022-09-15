// Copyright 2022 Mineiros GmbH
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

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestExpEval(t *testing.T) {
	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name        string
			layout      []string
			wd          string
			globals     []globalsBlock
			expr        string
			wantEval    runExpected
			wantPartial runExpected
		}
	)

	testcases := []testcase{
		{
			name: "boolean expression",
			expr: `true && false`,
			wantEval: runExpected{
				Stdout: addnl("false"),
			},
			wantPartial: runExpected{
				Stdout: addnl("true && false"),
			},
		},
		{
			name: "list expression",
			expr: `[1,1+0,1+1,1+2,3+2,5+3]`,
			wantEval: runExpected{
				Stdout: addnl("[1, 1, 2, 3, 5, 8]"),
			},
			wantPartial: runExpected{
				Stdout: addnl("[1, 1 + 0, 1 + 1, 1 + 2, 3 + 2, 5 + 3]"),
			},
		},
		{
			name: "simple funcalls",
			expr: `tm_upper("a")`,
			wantEval: runExpected{
				Stdout: addnl(`"A"`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "nested funcalls",
			expr: `tm_upper(tm_lower("A"))`,
			wantEval: runExpected{
				Stdout: addnl(`"A"`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "hierarchical globals evaluation",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("val", 49),
					),
				},
			},
			expr: `global.val+1`,
			wantEval: runExpected{
				Stdout: addnl(`50`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`49 + 1`),
			},
		},
		{
			name: "expression with multiple globals",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("num1", 10),
						Number("num2", 10),
					),
				},
			},
			expr: `global.num1 + global.num2`,
			wantEval: runExpected{
				Stdout: addnl(`20`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`10 + 10`),
			},
		},
		{
			name: "partial globals - not a stack, evaluating the global",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Str("val", "global string"),
						Expr("unknown", "terramate.stack.name"),
					),
				},
			},
			expr: `global.val`,
			wantEval: runExpected{
				Stdout: addnl(`"global string"`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"global string"`),
			},
		},
		{
			name: "partial globals - not a stack, evaluating the global.unknown",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Str("val", "global string"),
						Expr("unknown", "terramate.stack.name"),
					),
				},
			},
			expr: `global.unknown`,
			wantEval: runExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
			wantPartial: runExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
		},
		{
			name: "terramate stack metadata",
			layout: []string{
				"s:stack",
			},
			wd: "stack",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Str("val", "global string"),
					),
				},
			},
			expr: `"stack path: ${terramate.stack.path.absolute}"`,
			wantEval: runExpected{
				Stdout: addnl(`"stack path: /stack"`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"stack path: /stack"`),
			},
		},
		{
			name: "global + unknown",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("val", 1000),
					),
				},
			},
			expr: `unknown.num + global.val`,
			wantEval: runExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
			wantPartial: runExpected{
				Stdout: addnl(`unknown.num + 1000`),
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)

			s.BuildTree(tc.layout)

			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			assertRunResult(t, ts.run("experimental", "eval", tc.expr), tc.wantEval)
			assertRunResult(t, ts.run("experimental", "partial-eval", tc.expr), tc.wantPartial)
		})
	}
}

func TestGetConfigValue(t *testing.T) {
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
			expr    string
			want    runExpected
		}
	)

	testcases := []testcase{
		{
			name: "boolean expression",
			expr: `true || false`,
			want: runExpected{
				Status:      1,
				StderrRegex: "expected a variable accessor",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)

			s.BuildTree(tc.layout)

			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			assertRunResult(t, ts.run("experimental", "get-config-value", tc.expr), tc.want)
		})
	}
}

func addnl(s string) string { return s + "\n" }
