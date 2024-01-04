// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
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
			wantEval        RunExpected
			wantPartial     RunExpected
		}
	)

	testcases := []testcase{
		{
			name: "boolean expression",
			expr: `true && false`,
			wantEval: RunExpected{
				Stdout: addnl("false"),
			},
			wantPartial: RunExpected{
				Stdout: addnl("true && false"),
			},
		},
		{
			name: "list expression",
			expr: `[1,1+0,1+1,1+2,3+2,5+3]`,
			wantEval: RunExpected{
				Stdout: addnl("[1, 1, 2, 3, 5, 8]"),
			},
			wantPartial: RunExpected{
				Stdout: addnl("[1, 1 + 0, 1 + 1, 1 + 2, 3 + 2, 5 + 3]"),
			},
		},
		{
			name: "simple funcalls returning unquoted string",
			expr: `tm_upper("a")`,
			wantEval: RunExpected{
				Stdout: addnl(`A`),
			},
			wantPartial: RunExpected{
				Stdout: addnl(`"A"`),
			},
		},
		{
			name: "nested funcalls",
			expr: `tm_upper(tm_lower("A"))`,
			wantEval: RunExpected{
				Stdout: addnl(`A`),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl(`50`),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl(`20`),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl(`stack path: /stack`),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				StderrRegex: string(eval.ErrEval),
				Status:      1,
			},
			wantPartial: RunExpected{
				Stdout: addnl(`unknown.num + 1000`),
			},
		},
		{
			name: "set-global + eval",
			overrideGlobals: map[string]string{
				"value": `tm_upper("value")`,
			},
			expr: `global.value`,
			wantEval: RunExpected{
				Stdout: addnl("VALUE"),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl("root-mid-LEAF"),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl("BBB"),
			},
			wantPartial: RunExpected{
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
			wantEval: RunExpected{
				Stdout: addnl("AAA"),
			},
			wantPartial: RunExpected{
				Stdout: addnl(`"AAA"`),
			},
		},
		{
			// issue: https://github.com/terramate-io/terramate/issues/1327
			name: "regression check: globals containing percents",
			overrideGlobals: map[string]string{
				"x": `"E%2BkKhZA%3D"`,
			},
			expr: `global.x`,
			wantEval: RunExpected{
				Stdout: addnl(`E%2BkKhZA%3D`),
			},
			wantPartial: RunExpected{
				Stdout: addnl(`"E%2BkKhZA%3D"`),
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, false)
			s.BuildTree(tc.layout)
			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}

			test.WriteRootConfig(t, s.RootDir())
			ts := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))
			globalArgs := []string{}
			for globalName, globalExpr := range tc.overrideGlobals {
				globalArgs = append(globalArgs, "--global")
				globalArgs = append(globalArgs, fmt.Sprintf("%s=%s", globalName, globalExpr))
			}
			evalArgs := append([]string{"experimental", "eval"}, globalArgs...)
			partialArgs := append([]string{"experimental", "partial-eval"}, globalArgs...)
			evalArgs = append(evalArgs, tc.expr)
			partialArgs = append(partialArgs, tc.expr)
			AssertRunResult(t, ts.Run(evalArgs...), tc.wantEval)
			AssertRunResult(t, ts.Run(partialArgs...), tc.wantPartial)
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
			want            RunExpected
		}
	)

	testcases := []testcase{
		{
			name: "boolean expression",
			expr: `true || false`,
			want: RunExpected{
				Status:      1,
				StderrRegex: "expected a variable accessor",
			},
		},
		{
			name: "funcall expression",
			expr: `tm_upper("a")`,
			want: RunExpected{
				Status:      1,
				StderrRegex: "expected a variable accessor",
			},
		},
		{
			name: "unsupported rootname",
			expr: `local.value`,
			want: RunExpected{
				Status:      1,
				StderrRegex: "only terramate and global variables are supported",
			},
		},
		{
			name: "undefined global",
			expr: `global.value`,
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
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
			want: RunExpected{
				Stdout: addnl("1"),
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)

			for _, globalBlock := range tc.globals {
				path := filepath.Join(s.RootDir(), globalBlock.path)
				test.AppendFile(t, path, "globals.tm",
					globalBlock.add.String())
			}
			ts := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))
			globalArgs := []string{}
			for globalName, globalExpr := range tc.overrideGlobals {
				globalArgs = append(globalArgs, "--global")
				globalArgs = append(globalArgs, fmt.Sprintf("%s=%s", globalName, globalExpr))
			}
			args := append([]string{"experimental", "get-config-value"}, globalArgs...)
			args = append(args, tc.expr)
			AssertRunResult(t, ts.Run(args...), tc.want)
		})
	}
}

func addnl(s string) string { return s + "\n" }
