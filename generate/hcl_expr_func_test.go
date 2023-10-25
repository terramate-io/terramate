// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestHCLExpressionFunc(t *testing.T) {
	t.Parallel()
	// TODO(KATCIPIS): currently most behavior is tested on the genhcl pkg.
	// In the future tests could be moved here.
	testCodeGeneration(t, []testcase{
		{
			name: "not available on generate_hcl lets block",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Lets(
								Expr("expr", `tm_hcl_expression("test")`),
							),
							Content(
								Expr("value", `let.expr`),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "not available on generate_hcl condition",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Expr("condition", `tm_hcl_expression("test")`),
							Content(),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "not available on generate_hcl assert",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Assert(
								Expr("assertion", `tm_hcl_expression("true")`),
								Str("message", "msg"),
							),
							Content(),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "not available on generate_file condition",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateFile(
							Labels("test.txt"),
							Expr("condition", `tm_hcl_expression("test")`),
							Str("content", "content"),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "not available on generate_file assert",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateFile(
							Labels("test.txt"),
							Assert(
								Expr("assertion", `tm_hcl_expression("true")`),
								Str("message", "msg"),
							),
							Str("content", "content"),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			name: "not available on generate_file lets block",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: Doc(
						GenerateFile(
							Labels("test.txt"),
							Lets(
								Expr("content", `tm_hcl_expression("test")`),
							),
							Expr("content", "let.content"),
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
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
		{
			// There is no way to interpolate the expression on a string template
			name: "not available on generate_file content",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateFile(
						Labels("expr.txt"),
						Str("content", `generated: ${tm_hcl_expression("data")}`),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: project.NewPath("/stack"),
						},
						Error: errors.E(eval.ErrEval),
					},
				},
			},
		},
	})
}
