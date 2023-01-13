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
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestExpEval(t *testing.T) {
	t.Parallel()

	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name            string
			layout          []string
			wd              string
			globals         []globalsBlock
			overrideGlobals map[string]string
			expr            string
			wantEval        runExpected
			wantPartial     runExpected
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
			name: "simple funcalls returning unquoted string",
			expr: `tm_upper("a")`,
			wantEval: runExpected{
				Stdout: addnl(`A`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "nested funcalls",
			expr: `tm_upper(tm_lower("A"))`,
			wantEval: runExpected{
				Stdout: addnl(`A`),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "hierarchical globals evaluation",
			layout: []string{
				"s:stack",
			},
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("val", 49),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Number("val2", 1),
					),
				},
			},
			wd:   "stack",
			expr: `global.val+global.val2`,
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
			name: "partially successfully globals - not a stack, evaluating the undefined global",
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
				Stdout: addnl(`stack path: /stack`),
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
		{
			name: "set-global + eval",
			overrideGlobals: map[string]string{
				"value": `tm_upper("value")`,
			},
			expr: `global.value`,
			wantEval: runExpected{
				Stdout: addnl("VALUE"),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"VALUE"`),
			},
		},
		{
			name: "setting multiple globals with dependency",
			overrideGlobals: map[string]string{
				"leaf": `tm_upper("leaf")`,
				"mid":  `"mid-${global.leaf}"`,
				"root": `"root-${global.mid}"`,
			},
			expr: `global.root`,
			wantEval: runExpected{
				Stdout: addnl("root-mid-LEAF"),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"root-mid-LEAF"`),
			},
		},
		{
			name: "override defined global",
			overrideGlobals: map[string]string{
				"value": `"BBB"`,
			},
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Str("value", "AAA"),
					),
				},
			},
			expr: `global.value`,
			wantEval: runExpected{
				Stdout: addnl("BBB"),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"BBB"`),
			},
		},
		{
			name: "override underspecified global",
			overrideGlobals: map[string]string{
				"value": `"AAA"`,
			},
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						// this would be a hard fail if the --global is not provided
						Expr("value", `something.that.does.not.exists`),
					),
				},
			},
			expr: `global.value`,
			wantEval: runExpected{
				Stdout: addnl("AAA"),
			},
			wantPartial: runExpected{
				Stdout: addnl(`"AAA"`),
			},
		},
		{
			name: "underspecified globals still fail",
			overrideGlobals: map[string]string{
				"value": `"AAA"`,
			},
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						// this would be a hard fail if the --global is not provided
						Expr("value", `something.that.does.not.exists`),
						Expr("another_value", `something.that.does.not.exists`),
					),
				},
			},
			expr: `global.value`,
			wantEval: runExpected{
				StderrRegex: string(globals.ErrEval),
				Status:      1,
			},
			wantPartial: runExpected{
				StderrRegex: string(globals.ErrEval),
				Status:      1,
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			globalArgs := []string{}
			for globalName, globalExpr := range tc.overrideGlobals {
				globalArgs = append(globalArgs, "--global")
				globalArgs = append(globalArgs, fmt.Sprintf("%s=%s", globalName, globalExpr))
			}
			evalArgs := append([]string{"experimental", "eval"}, globalArgs...)
			partialArgs := append([]string{"experimental", "partial-eval"}, globalArgs...)
			evalArgs = append(evalArgs, tc.expr)
			partialArgs = append(partialArgs, tc.expr)
			assertRunResult(t, ts.run(evalArgs...), tc.wantEval)
			assertRunResult(t, ts.run(partialArgs...), tc.wantPartial)
		})
	}
}

func TestGetConfigValue(t *testing.T) {
	t.Parallel()

	type (
		globalsBlock struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name            string
			layout          []string
			wd              string
			globals         []globalsBlock
			overrideGlobals map[string]string
			expr            string
			want            runExpected
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
		{
			name: "funcall expression",
			expr: `tm_upper("a")`,
			want: runExpected{
				Status:      1,
				StderrRegex: "expected a variable accessor",
			},
		},
		{
			name: "unsupported rootname",
			expr: `local.value`,
			want: runExpected{
				Status:      1,
				StderrRegex: "only terramate and global variables are supported",
			},
		},
		{
			name: "undefined global",
			expr: `global.value`,
			want: runExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
		},
		{
			name: "terramate.stacks.list works on any directory inside rootdir",
			layout: []string{
				`s:stacks/stack1`,
				`s:stacks/stack2`,
				`d:dir/outside/stacks/hierarchy`,
			},
			wd:   "dir/outside/stacks/hierarchy",
			expr: `terramate.stacks.list`,
			want: runExpected{
				Stdout: addnl(`["/stacks/stack1", "/stacks/stack2"]`),
			},
		},
		{
			name: "get global.val",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("val", 1000),
					),
				},
			},
			expr: `global.val`,
			want: runExpected{
				Stdout: addnl("1000"),
			},
		},
		{
			name: "get deep value in global object",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Expr("object", `{
							level1 = {
								level2 = {
									level3 = {
										hello = "hello"
									}
								}
							}
						}`),
					),
				},
			},
			expr: `global.object.level1.level2.level3.hello`,
			want: runExpected{
				Stdout: addnl(`hello`),
			},
		},
		{
			name: "get-config-value with overridden globals",
			globals: []globalsBlock{
				{
					path: "/",
					add: Globals(
						Number("val", 1000),
					),
				},
			},
			overrideGlobals: map[string]string{
				"val": "1",
			},
			expr: `global.val`,
			want: runExpected{
				Stdout: addnl("1"),
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)

			s.BuildTree(tc.layout)

			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			globalArgs := []string{}
			for globalName, globalExpr := range tc.overrideGlobals {
				globalArgs = append(globalArgs, "--global")
				globalArgs = append(globalArgs, fmt.Sprintf("%s=%s", globalName, globalExpr))
			}
			args := append([]string{"experimental", "get-config-value"}, globalArgs...)
			args = append(args, tc.expr)
			assertRunResult(t, ts.run(args...), tc.want)
		})
	}
}

func addnl(s string) string { return s + "\n" }
