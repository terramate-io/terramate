// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/generate/genhcl"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
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
								Expr("type", "any"),
								Bool("sensitive", true),
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
						Bool("sensitive", false),
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
								Expr("type", "any"),
								Bool("sensitive", false),
							),
							Block("variable",
								Labels("var_name2"),
								Expr("type", "any"),
								Bool("sensitive", true),
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
								Bool("sensitive", true),
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
						Bool("sensitive", false),
					),
				},
				{
					path:     "/stacks/stack-1",
					filename: "output2.tm",
					add: Output(
						Labels("var_name2"),
						Str("backend", "name"),
						Expr("value", "module.something2"),
						Bool("sensitive", true),
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
								Bool("sensitive", false),
							),
							Block("output",
								Labels("var_name2"),
								Expr("value", "module.something2"),
								Bool("sensitive", true),
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
							Bool("sensitive", true),
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
							Bool("sensitive", false),
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
								Expr("type", "any"),
								Bool("sensitive", true),
							),
							Block("variable",
								Labels("var_input2"),
								Expr("type", "any"),
								Bool("sensitive", true),
							),
							Block("variable",
								Labels("var_input3"),
								Expr("type", "any"),
								Bool("sensitive", false),
							),
							Block("variable",
								Labels("var_input4"),
								Expr("type", "any"),
								Bool("sensitive", true),
							),
							Block("output",
								Labels("var_output1"),
								Expr("value", "module.something1"),
								Bool("sensitive", true),
							),
							Block("output",
								Labels("var_output2"),
								Expr("value", "module.something2"),
								Bool("sensitive", true),
							),
							Block("output",
								Labels("var_output3"),
								Expr("value", "module.something3"),
								Bool("sensitive", true),
							),
							Block("output",
								Labels("var_output4"),
								Expr("value", "module.something4"),
								Bool("sensitive", true),
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

func TestSharingOrphanedFilesAreDeleted(t *testing.T) {
	t.Parallel()
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		`s:s1`,
		`s:s2`,
		`f:exp.tm:` + Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
		`f:backend.tm:` + Block("sharing_backend",
			Labels("terraform"),
			Expr("type", "terraform"),
			Str("filename", "sharing.tf"),
			Command("echo", "whatever"),
		).String(),
		`f:s1/outputs.tm:` + Output(
			Labels("name"),
			Expr("value", "module.test"),
			Str("backend", "terraform"),
		).String(),
		`f:s2/inputs.tm:` + Input(
			Labels("name"),
			Expr("value", "outputs.name.value"),
			Str("backend", "terraform"),
			Str("from_stack_id", "whatever"),
		).String(),
	})
	s.Generate()
	expectedOutput := genhcl.Header(genhcl.DefaultComment) + Block("output",
		Labels("name"),
		Expr("value", "module.test"),
		Bool("sensitive", true),
	).String() + "\n"
	gotOutput := s.RootEntry().ReadFile("s1/sharing.tf")
	assert.EqualStrings(t, expectedOutput, string(gotOutput))

	expectedInput := genhcl.Header(genhcl.DefaultComment) + Block("variable",
		Labels("name"),
		Expr("type", "any"),
		Bool("sensitive", true),
	).String() + "\n"
	gotInput := s.RootEntry().ReadFile("s2/sharing.tf")
	assert.EqualStrings(t, expectedInput, string(gotInput))

	s.RootEntry().RemoveFile("s1/outputs.tm")
	s.Generate()
	// s1/sharing.tf must be deleted
	assertFileDeleted(t, "s1/sharing.tf")
	gotInput = s.RootEntry().ReadFile("s2/sharing.tf")
	assert.EqualStrings(t, expectedInput, string(gotInput))

	s.RootEntry().RemoveFile("s2/inputs.tm")
	assertFileDeleted(t, "s1/sharing.tf")
	assertFileDeleted(t, "s2/sharing.tf")
}

func assertFileDeleted(t *testing.T, name string) {
	_, err := os.Lstat(name)
	if !os.IsNotExist(err) {
		t.Fatalf("file %s is still present", name)
	}
}
