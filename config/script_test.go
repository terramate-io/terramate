// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"

	. "github.com/terramate-io/terramate/test/hclutils"
	. "github.com/terramate-io/terramate/test/hclutils/info"
	"github.com/zclconf/go-cty/cty"
)

func TestScriptEval(t *testing.T) {
	t.Parallel()

	makeAttribute := func(t *testing.T, name string, expr string) *ast.Attribute {
		t.Helper()
		return &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: name,
				Expr: test.NewExpr(t, expr),
			},
		}
	}

	makeCommand := func(t *testing.T, expr string) *hcl.Command {
		t.Helper()
		attr := makeAttribute(t, "command", expr)
		parsed := hcl.Command(*attr)
		return &parsed
	}

	makeCommands := func(t *testing.T, expr string) *hcl.Commands {
		t.Helper()
		attr := makeAttribute(t, "commands", expr)
		parsed := hcl.Commands(*attr)
		return &parsed
	}

	labels := []string{"some", "label"}

	type testcase struct {
		name    string
		script  hcl.Script
		globals map[string]cty.Value
		want    config.Script
		wantErr error
	}

	tcases := []testcase{
		{
			name: "no description attribute",
			script: hcl.Script{
				Labels: labels,
			},
			want: config.Script{
				Labels: labels,
			},
		},
		{
			name: "description attribute wrong type",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `666`),
			},
			wantErr: errors.E(config.ErrScriptInvalidType),
		},
		{
			name: "description attribute with functions",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `tm_upper("some description")`),
			},
			want: config.Script{
				Labels:      labels,
				Description: "SOME DESCRIPTION",
			},
		},
		{
			name: "description attribute with functions and globals",
			script: hcl.Script{
				Range:       Range("script.tm", Start(1, 1, 0), End(3, 2, 37)),
				Labels:      labels,
				Description: makeAttribute(t, "description", `tm_upper(global.some_string_var)`),
			},
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Description: "TERRAMATE",
			},
		},
		{
			name: "name attribute with wrong type",
			script: hcl.Script{
				Labels:      labels,
				Name:        makeAttribute(t, "name", `666`),
				Description: makeAttribute(t, "description", `"some desc"`),
			},
			wantErr: errors.E(config.ErrScriptInvalidType),
		},
		{
			name: "name attribute with string",
			script: hcl.Script{
				Labels:      labels,
				Name:        makeAttribute(t, "name", `"some name"`),
				Description: makeAttribute(t, "description", `"some desc"`),
			},
			want: config.Script{
				Labels:      labels,
				Name:        "some name",
				Description: "some desc",
			},
		},
		{
			name: "name attribute exceeds maximum allowed characters - truncation",
			script: hcl.Script{
				Labels:      labels,
				Name:        makeAttribute(t, "name", `"`+strings.Repeat("A", 150)+`"`),
				Description: makeAttribute(t, "description", `"some desc"`),
			},
			want: config.Script{
				Labels:      labels,
				Name:        strings.Repeat("A", 128),
				Description: "some desc",
			},
		},
		{
			name: "name attribute with functions",
			script: hcl.Script{
				Labels:      labels,
				Name:        makeAttribute(t, "name", `tm_upper("some name")`),
				Description: makeAttribute(t, "description", `"some desc"`),
			},
			want: config.Script{
				Labels:      labels,
				Name:        "SOME NAME",
				Description: "some desc",
			},
		},
		{
			name: "name attribute with interpolation, functions and globals",
			script: hcl.Script{
				Range:       Range("script.tm", Start(1, 1, 0), End(3, 2, 37)),
				Labels:      labels,
				Name:        makeAttribute(t, "name", `"my name is ${tm_upper(global.name_var)}!!!"`),
				Description: makeAttribute(t, "description", `"some desc"`),
			},
			globals: map[string]cty.Value{
				"name_var": cty.StringVal("terramate"),
			},
			want: config.Script{
				Labels:      labels,
				Name:        "my name is TERRAMATE!!!",
				Description: "some desc",
			},
		},
		{
			name: "command attribute wrong type",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `"echo"`),
					},
				},
			},
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "command attribute wrong element type",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `["echo", 666]`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "command attribute with functions and globals",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `["echo", tm_upper("hello ${global.some_string_var}")]`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `true ? tm_concat(["echo"], ["something"]) : []`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `true ? tm_concat([["echo", "something"]], [["echo", "other", "thing"]]) : []`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `["${global.some_command_name}", "--version"]`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `"echo"`),
					},
				},
			},
			globals: map[string]cty.Value{
				"some_string_var": cty.StringVal("terramate"),
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommands),
		},
		{
			name: "commands attribute wrong element type",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello"],
							666,
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommands),
		},
		{
			name: "commands evaluating to empty list",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `[]`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "commands item evaluating to empty list",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `[
							["echo", "hello"],
							[],
							["echo", "other"]
						]`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "command evaluating to empty list",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Command: makeCommand(t, `[]`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptEmptyCmds),
		},
		{
			name: "commands attribute with functions and globals",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", tm_upper("hello ${global.some_string_var}")],
							["stat", "."],
						  ]
						`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Name: makeAttribute(t, "name", `"job name"`),
						Commands: makeCommands(t, `
						  [
							["echo", tm_upper("hello ${global.some_string_var}")],
							["stat", "."],
						  ]
						`),
					},
					{
						Commands: makeCommands(t, `
						  [
							["echo", tm_upper("hello ${global.some_string_var}")],
							["ls", "-l"],
						  ]
						`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some desc"`),
				Jobs: []*hcl.ScriptJob{
					{
						Name: makeAttribute(t, "name", `"`+strings.Repeat("A", 150)+`"`),
						Commands: makeCommands(t, `
						  [
							["echo", "hello"],
						  ]
						`),
					},
				},
			},
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
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some desc"`),
				Jobs: []*hcl.ScriptJob{
					{
						Description: makeAttribute(t, "description", `"`+strings.Repeat("A", config.MaxScriptDescRunes+100)+`"`),
						Commands: makeCommands(t, `
						  [
							["echo", "hello"],
						  ]
						`),
					},
				},
			},
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
			name: "command options with cloud_sync_deployment and cloud_sync_preview",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deployment = true
								cloud_sync_preview = true
								cloud_sync_terraform_plan_file = "plan_a"
							}],
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "command options with invalid cloud_sync_layer",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_preview = true
								cloud_sync_terraform_plan_file = "plan_a"
								cloud_sync_layer = "a+b"
							}],
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "command options with cloud_sync_preview + planfile + layer",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_preview = true
								cloud_sync_terraform_plan_file = "plan_a"
								cloud_sync_layer = "staging"
							}],
						  ]
						`),
					},
				},
			},
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
									CloudSyncTerraformPlan: "plan_a",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "command options",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deployment = false
								cloud_sync_terraform_plan_file = "plan_a"
							}],
						  ]
						`),
					},
					{
						Command: makeCommand(t, `
							["echo", "hello", {
								cloud_sync_deployment = true
								cloud_sync_terraform_plan_file = "plan_b"
							}]
						`),
					},
				},
			},
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
									CloudSyncTerraformPlan: "plan_a",
								},
							},
						},
					},
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"echo", "hello"},
							Options: &config.ScriptCmdOptions{
								CloudSyncDeployment:    true,
								CloudSyncTerraformPlan: "plan_b",
							},
						},
					},
				},
			},
		},
		{
			name: "command options with planfile + terragrunt",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deployment = true
								cloud_sync_terraform_plan_file = "plan_a"
								terragrunt = true
							}],
						  ]
						`),
					},
				},
			},
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
									CloudSyncTerraformPlan: "plan_a",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid command option",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deploymenttttttt = false
							}],
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "invalid command options object",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", ["list"]],
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeCommand),
		},
		{
			name: "invalid command option type",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deployment = "false"
							}],
						  ]
						`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
		{
			name: "multiple cloud_sync_deployments",
			script: hcl.Script{
				Labels:      labels,
				Description: makeAttribute(t, "description", `"some description"`),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
							  [
								["echo", "hello", {
									cloud_sync_deployment = true
									cloud_sync_terraform_plan_file = "plan_a"
								}],
							  ]
							`),
					},
					{
						Command: makeCommand(t, `
								["echo", "hello", {
									cloud_sync_deployment = true
									cloud_sync_terraform_plan_file = "plan_a"
								}]
							`),
					},
				},
			},
			wantErr: errors.E(config.ErrScriptInvalidCmdOptions),
		},
	}

	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			hclctx := eval.NewContext(stdlib.Functions(test.TempDir(t)))
			hclctx.SetNamespace("global", tcase.globals)

			got, err := config.EvalScript(hclctx, tcase.script)
			assert.IsError(t, err, tcase.wantErr)
			// ignoring info.Range comparisons for now
			if diff := cmp.Diff(tcase.want, got, cmpopts.IgnoreUnexported(info.Range{})); diff != "" {
				t.Fatalf("unexpected result\n%s", diff)
			}
		})
	}
}
