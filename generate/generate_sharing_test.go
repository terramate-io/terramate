// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestGenerateSharing(t *testing.T) {
	t.Parallel()

	enableSharingExperiment := Terramate(
		Config(
			Experiments(hcl.SharingIsCaringExperimentName),
		),
	)

	testCodeGeneration(t, []testcase{
		{
			name: "no input/output generates no file",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
			},
		},
		{
			name: "single input generated",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Input(
						Labels("var_name"),
						Str("backend", "name"),
						Expr("value", "outputs.var_name"),
						Str("from_stack_id", "abc"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.tf": Doc(
							Block("variable",
								Labels("var_name"),
								Expr("type", "string"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.tf"},
					},
				},
			},
		},
		{
			name: "multiple inputs generated from different files",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "input1.tm",
					add: Input(
						Labels("var_name"),
						Str("backend", "name"),
						Expr("value", "outputs.var_name"),
						Str("from_stack_id", "abc"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "input2.tm",
					add: Input(
						Labels("var_name2"),
						Str("backend", "name"),
						Expr("value", "outputs.var_name"),
						Str("from_stack_id", "abc2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.tf": Doc(
							Block("variable",
								Labels("var_name"),
								Expr("type", "string"),
							),
							Block("variable",
								Labels("var_name2"),
								Expr("type", "string"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.tf"},
					},
				},
			},
		},
		{
			name: "single output generated",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "output.tm",
					add: Output(
						Labels("var_name"),
						Str("backend", "name"),
						Expr("value", "module.something"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.tf": Doc(
							Block("output",
								Labels("var_name"),
								Expr("value", "module.something"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.tf"},
					},
				},
			},
		},
		{
			name: "multiple output generated",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "output1.tm",
					add: Output(
						Labels("var_name1"),
						Str("backend", "name"),
						Expr("value", "module.something1"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "output2.tm",
					add: Output(
						Labels("var_name2"),
						Str("backend", "name"),
						Expr("value", "module.something2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.tf": Doc(
							Block("output",
								Labels("var_name1"),
								Expr("value", "module.something1"),
							),
							Block("output",
								Labels("var_name2"),
								Expr("value", "module.something2"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.tf"},
					},
				},
			},
		},
		{
			name: "mixed input and output",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add:  enableSharingExperiment,
				},
				{
					path: "/",
					add: Block("sharing_backend",
						Labels("name"),
						Expr("type", "terraform"),
						Expr("command", `["echo"]`),
						Str("filename", "test.tf"),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "file1.tm",
					add: Doc(
						Output(
							Labels("var_output1"),
							Str("backend", "name"),
							Expr("value", "module.something1"),
						),
						Input(
							Labels("var_input1"),
							Str("backend", "name"),
							Expr("value", "outputs.something1"),
							Str("from_stack_id", "abc"),
						),
						Output(
							Labels("var_output2"),
							Str("backend", "name"),
							Expr("value", "module.something2"),
						),
						Input(
							Labels("var_input2"),
							Str("backend", "name"),
							Expr("value", "outputs.something2"),
							Str("from_stack_id", "abc"),
						),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "file2.tm",
					add: Doc(
						Output(
							Labels("var_output3"),
							Str("backend", "name"),
							Expr("value", "module.something3"),
						),
						Input(
							Labels("var_input3"),
							Str("backend", "name"),
							Expr("value", "outputs.something3"),
							Str("from_stack_id", "abc"),
						),
						Output(
							Labels("var_output4"),
							Str("backend", "name"),
							Expr("value", "module.something4"),
						),
						Input(
							Labels("var_input4"),
							Str("backend", "name"),
							Expr("value", "outputs.something4"),
							Str("from_stack_id", "abc"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test.tf": Doc(
							Block("variable",
								Labels("var_input1"),
								Expr("type", "string"),
							),
							Block("variable",
								Labels("var_input2"),
								Expr("type", "string"),
							),
							Block("variable",
								Labels("var_input3"),
								Expr("type", "string"),
							),
							Block("variable",
								Labels("var_input4"),
								Expr("type", "string"),
							),
							Block("output",
								Labels("var_output1"),
								Expr("value", "module.something1"),
							),
							Block("output",
								Labels("var_output2"),
								Expr("value", "module.something2"),
							),
							Block("output",
								Labels("var_output3"),
								Expr("value", "module.something3"),
							),
							Block("output",
								Labels("var_output4"),
								Expr("value", "module.something4"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test.tf"},
					},
				},
			},
		},
	})
}
