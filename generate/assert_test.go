// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package generate_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateAssert(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "no generate blocks with success assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", true),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{},
		},
		{
			name: "assert blocks with eval failures",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Expr("assertion", "unknown.ref"),
						Expr("message", "unknown.ref"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(eval.ErrEval),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "no generate blocks with failed assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
				},
			},
		},
		{
			name: "failed assertion using tm_version_match()",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Expr("assertion", `tm_version_match(terramate.version, "< 0.2")`),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
				},
			},
		},
		{
			name: "generate blocks ignored on failed assertion",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.E(generate.ErrAssertion),
					},
				},
			},
		},
		{
			name: "failed assertion message contents",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.E(generate.ErrAssertion, "/stacks/terramate.tm.hcl:3,15-20: msg"),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.E(generate.ErrAssertion, "/stacks/terramate.tm.hcl:3,15-20: msg"),
					},
				},
			},
		},
		{
			name: "generates code when failed assertion is a warning",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Expr("message", `"msg"`),
						Bool("warning", true),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stacks", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stacks", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.hcl", "test.txt"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"test.hcl", "test.txt"},
					},
				},
			},
		},
		{
			name: "failed assertions on all levels",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
				{
					path: "/stacks",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Assert(
						Bool("assertion", false),
						Str("message", "msg"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-1"),
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
						),
					},
					{
						Result: generate.Result{
							Dir: project.NewPath("/stacks/stack-2"),
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
						),
					},
				},
			},
		},
	})
}

func TestGenerateAssertInsideGenerateBlocks(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "success assertion",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stack", "test"),
							),
							Assert(
								Bool("assertion", true),
								Str("message", "msg"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
							Assert(
								Bool("assertion", true),
								Str("message", "msg"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stack", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test.hcl", "test.txt"},
					},
				},
			},
		},
		{
			name: "success assertion using tm_version_match()",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stack", "test"),
							),
							Assert(
								Expr("assertion", `tm_version_match(terramate.version, ">= 0.2", {allow_prereleases = true})`),
								Str("message", "msg"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stack", "test"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test.hcl"},
					},
				},
			},
		},
		{
			name: "all blocks ignored if a single block fails assertion",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
						GenerateHCL(
							Labels("test2.hcl"),
							Content(
								Str("stacks", "test"),
							),
							Assert(
								Bool("assertion", false),
								Str("message", "msg"),
							),
						),
						GenerateFile(
							Labels("test2.txt"),
							Str("content", "test"),
							Assert(
								Bool("assertion", false),
								Str("message", "msg"),
							),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stack"),
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion),
							errors.E(generate.ErrAssertion),
						),
					},
				},
			},
		},
		{
			name: "generates code when failed assertion is a warning",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stack", "test"),
							),
							Assert(
								Bool("assertion", false),
								Str("message", "msg"),
								Bool("warning", true),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
							Assert(
								Bool("assertion", false),
								Str("message", "msg"),
								Bool("warning", true),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test.hcl": Doc(
							Str("stack", "test"),
						),
						"test.txt": stringer("test"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test.hcl", "test.txt"},
					},
				},
			},
		},
		{
			name: "failed assertion message contents",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stack", "test"),
							),
							Assert(
								Bool("assertion", false),
								Str("message", "msg"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
							Assert(
								Bool("assertion", false),
								Str("message", "msg2"),
							),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stack"),
						},
						Error: errors.L(
							errors.E(generate.ErrAssertion, "/stack/terramate.tm.hcl:7,17-22: msg"),
							errors.E(generate.ErrAssertion, "/stack/terramate.tm.hcl:14,17-22: msg2"),
						),
					},
				},
			},
		},
	})
}
