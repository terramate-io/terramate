// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

type scriptTestcase struct {
	name    string
	config  fmt.Stringer
	globals map[string]cty.Value
	want    config.Script
	wantErr error
}

func TestScriptEval(t *testing.T) {
	t.Parallel()

	labels := []string{"some", "label"}

	tcases := []scriptTestcase{
		{
			name: "no description attribute",
			config: Script(
				Labels(labels...),
				Block("job",
					Command("echo", "hello"),
				),
			),
			want: config.Script{
				Labels: labels,
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "description attribute wrong type",
			config: Script(
				Labels(labels...),
				Number("description", 666),
				Block("job", Command("echo", "hello")),
			),
			wantErr: errors.E(config.ErrScriptInvalidType),
		},
		{
			name: "description attribute with functions",
			config: Script(
				Labels(labels...),
				Expr("description", `tm_upper("some description")`),
				Block("job", Command("echo", "hello")),
			),
			want: config.Script{
				Labels:      labels,
				Description: "SOME DESCRIPTION",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "description attribute with functions and globals",
			config: Script(
				Labels(labels...),
				Expr("description", `tm_upper(global.some_string_var)`),
				Block("job", Command("echo", "hello")),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "TERRAMATE",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "name attribute with wrong type",
			config: Script(
				Labels(labels...),
				Number("name", 666),
				Str("description", "some desc"),
				Block("job", Command("echo", "hello")),
			),
			wantErr: errors.E(config.ErrScriptInvalidType),
		},
		{
			name: "name attribute with string",
			config: Script(
				Labels(labels...),
				Str("name", "some name"),
				Str("description", "some desc"),
				Block("job", Command("echo", "hello")),
			),
			want: config.Script{
				Labels:      labels,
				Name:        "some name",
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "name attribute exceeds maximum allowed characters - truncation",
			config: Script(
				Labels(labels...),
				Str("name", strings.Repeat("A", 150)),
				Str("description", "some desc"),
				Block("job", Command("echo", "hello")),
			),
			want: config.Script{
				Labels:      labels,
				Name:        strings.Repeat("A", 128),
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "name attribute with functions",
			config: Script(
				Labels(labels...),
				Expr("name", `tm_upper("some name")`),
				Str("description", "some desc"),
				Block("job", Command("echo", "hello")),
			),
			want: config.Script{
				Labels:      labels,
				Name:        "SOME NAME",
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "name attribute with interpolation, functions and globals",
			config: Script(
				Labels(labels...),
				Expr("name", `"my name is ${tm_upper(global.name_var)}!!!"`),
				Str("description", "some desc"),
				Block("job", Command("echo", "hello")),
			),
			globals: map[string]cty.Value{
				"name_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Name:        "my name is TERRAMATE!!!",
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{Cmd: &config.ScriptCmd{Args: []string{"echo", "hello"}}},
				},
			},
		},
		{
			name: "command attribute wrong type - must be a list",
			config: Script(
				Labels(labels...),
				Str("description", "some desc"),
				Block("job",
					Str("command", "echo"),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "command attribute wrong element type",
			config: Script(
				Labels(labels...),
				Str("description", "some desc"),
				Block("job",
					Expr("command", `["echo", 666]`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "command attribute with functions and globals",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("command", `["echo", tm_upper("hello ${global.some_string_var}")]`),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"echo", "HELLO TERRAMATE"},
						},
					},
				},
			},
		},
		{
			name: "command attribute with list type",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("command", `true ? tm_concat(["echo"], ["something"]) : []`)),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"echo", "something"},
						},
					},
				},
			},
		},
		{
			name: "commands attribute with list type",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `true ? tm_concat([["echo", "something"]], [["echo", "other", "thing"]]) : []`),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "something"},
							},
							{
								Args: []string{"echo", "other", "thing"},
							},
						},
					},
				},
			},
		},
		{
			name: "command with first item interpolated",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("command", `["${global.some_command_name}", "--version"]`),
				),
			),
			globals: map[string]cty.Value{
				"some_command_name": cty.StringVal("ls"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"ls", "--version"},
						},
					},
				},
			},
		},
		{
			name: "commands attribute wrong type",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Str("commands", "echo"),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommands),
		},
		{
			name: "commands attribute wrong element type",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Str("commands", "test"),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidTypeCommands),
		},
		{
			name: "commands evaluating to empty list",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[]`),
				),
			),
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "commands item evaluating to empty list",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello"],
								[],
								["echo", "other"]
							]`),
				),
			),
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "command evaluating to empty list",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("command", `[]`),
				),
			),
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "commands attribute with functions and globals",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", tm_upper("hello ${global.some_string_var}")],
								["stat", "."],
							  ]
							`),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{Args: []string{"echo", "HELLO TERRAMATE"}},
							{Args: []string{"stat", "."}},
						},
					},
				},
			},
		},
		{
			name: "multiple jobs",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Str("name", "job name"),
					Expr("commands", `[
								["echo", tm_upper("hello ${global.some_string_var}")],
								["stat", "."],
							  ]`),
				),
				Block("job",
					Expr("commands", `[
								["echo", tm_upper("hello ${global.some_string_var}")],
								["ls", "-l"],
							  ]
							`),
				),
			),
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Name: "job name",
						Cmds: []*config.ScriptCmd{
							{Args: []string{"echo", "HELLO TERRAMATE"}},
							{Args: []string{"stat", "."}},
						},
					},
					{
						Cmds: []*config.ScriptCmd{
							{Args: []string{"echo", "HELLO TERRAMATE"}},
							{Args: []string{"ls", "-l"}},
						},
					},
				},
			},
		},
		{
			name: "job.name attribute exceeds maximum allowed characters - truncation",
			config: Script(
				Labels(labels...),
				Str("description", "some desc"),
				Block("job",
					Str("name", strings.Repeat("A", 150)),
					Expr("commands", `[["echo", "hello"]]`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{
						Name: strings.Repeat("A", 128),
						Cmds: []*config.ScriptCmd{
							{Args: []string{"echo", "hello"}},
						},
					},
				},
			},
		},
		{
			name: "job.description attribute exceeds maximum allowed characters - truncation",
			config: Script(
				Labels(labels...),
				Str("description", "some desc"),
				Block("job",
					Str("description", strings.Repeat("A", config.MaxScriptDescRunes+100)),
					Expr("commands", `[["echo", "hello"]]`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some desc",
				Jobs: []config.ScriptJob{
					{
						Description: strings.Repeat("A", 1000),
						Cmds: []*config.ScriptCmd{
							{Args: []string{"echo", "hello"}},
						},
					},
				},
			},
		},
		{
			name: "command options with sync_deployment and sync_preview",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_deployment = true
									sync_preview = true
									terraform_plan_file = "plan_a"
								}],
							  ]
							`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "command options with invalid layer",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_preview = true
									terraform_plan_file = "plan_a"
									layer = "a+b"
								}],
							  ]
							`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "command options with sync_preview + planfile + layer",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_preview = true
									terraform_plan_file = "plan_a"
									layer = "staging"
								}],
							  ]
							`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "hello"},
								Options: &config.ScriptCmdOptions{
									CloudSyncDeployment:    false,
									CloudSyncPreview:       true,
									CloudSyncLayer:         "staging",
									CloudTerraformPlanFile: "plan_a",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "command options",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_deployment = false
									terraform_plan_file = "plan_a"
								}],
							  ]
							`),
				),
				Block("job",
					Expr("command", `["echo", "hello", {
									sync_deployment = true
									terraform_plan_file = "plan_b"
								}]`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "hello"},
								Options: &config.ScriptCmdOptions{
									CloudSyncDeployment:    false,
									CloudTerraformPlanFile: "plan_a",
								},
							},
						},
					},
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"echo", "hello"},
							Options: &config.ScriptCmdOptions{
								CloudSyncDeployment:    true,
								CloudTerraformPlanFile: "plan_b",
							},
						},
					},
				},
			},
		},
		{
			name: "command options with planfile + terragrunt + custom timeout",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_deployment = true
									terraform_plan_file = "plan_a"
									terragrunt = true
									plan_render_timeout = 10
								}],
							  ]
							`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "hello"},
								Options: &config.ScriptCmdOptions{
									CloudSyncDeployment:    true,
									UseTerragrunt:          true,
									CloudTerraformPlanFile: "plan_a",
									CloudPlanRenderTimeout: 10 * time.Second,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid command option",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_deploymenttttttt = false
								}],
							  ]
							`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "invalid command options object",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", ["list"]],
							  ]
							`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "invalid command option type - option must be boolean",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
								["echo", "hello", {
									sync_deployment = "false"
								}],
							  ]
							`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "multiple sync_deployments",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Block("job",
					Expr("commands", `[
									["echo", "hello", {
										sync_deployment = true
										terraform_plan_file = "plan_a"
									}],
								  ]`),
				),
				Block("job",
					Expr("commands", `[["echo", "hello", {
										sync_deployment = true
										terraform_plan_file = "plan_a"
									}]]`),
				),
			),
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
	}

	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			testScriptEval(t, tcase)
		})
	}
}

func testScriptEval(t *testing.T, tcase scriptTestcase) {
	t.Helper()
	tempdir := test.TempDir(t)
	test.AppendFile(t, tempdir, "stack.tm", Block("stack").String())
	test.AppendFile(t, tempdir, "script.tm", tcase.config.String())
	test.AppendFile(t, tempdir, "terramate.tm", Terramate(
		Config(
			Expr("experiments", `["scripts"]`),
		),
	).String())

	cfg, err := config.LoadRoot(tempdir, false)
	if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
		errtest.Assert(t, err, tcase.wantErr)
		return
	}

	assert.NoError(t, err)

	rootTree, ok := cfg.Lookup(project.NewPath("/"))
	if !ok {
		panic("root tree not found")
	}

	st, err := rootTree.Stack()
	assert.NoError(t, err)
	hclctx := eval.NewContext(stdlib.Functions(tempdir, []string{}))
	hclctx.SetNamespace("global", tcase.globals)
	runtime := cfg.Runtime()
	runtime.Merge(st.RuntimeValues(cfg))
	hclctx.SetNamespace("terramate", runtime)

	if len(rootTree.Node.Scripts) != 1 {
		panic("test expects one script")
	}
	got, err := config.EvalScript(hclctx, *rootTree.Node.Scripts[0])
	assert.IsError(t, err, tcase.wantErr)
	// ignoring info.Range comparisons for now
	if diff := cmp.Diff(tcase.want, got, cmpopts.IgnoreUnexported(info.Range{})); diff != "" {
		t.Fatalf("unexpected result\n%s", diff)
	}
}
