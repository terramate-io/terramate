// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
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

	makeAttribute := func(t *testing.T, name string, expr string) ast.Attribute {
		t.Helper()
		return ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: name,
				Expr: test.NewExpr(t, expr),
			},
		}
	}

	makeCommand := func(t *testing.T, expr string) *hcl.Command {
		parsed := hcl.Command(makeAttribute(t, "command", expr))
		return &parsed
	}

	makeCommands := func(t *testing.T, expr string) *hcl.Commands {
		parsed := hcl.Commands(makeAttribute(t, "commands", expr))
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
			name: "description attribute wrong type",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `666`)),
			},
			wantErr: errors.E(config.ErrScriptInvalidTypeDesc),
		},
		{
			name: "description attribute with functions",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `tm_upper("some description")`)),
			},
			want: config.Script{
				Labels:      labels,
				Description: "SOME DESCRIPTION",
			},
		},
		{
			name: "description attribute with functions and globals",
			script: hcl.Script{
				Range:  Range("script.tm", Start(1, 1, 0), End(3, 2, 37)),
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `tm_upper(global.some_string_var)`)),
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
			name: "command attribute wrong type",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
			name: "command with first item interpolated",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
				Jobs: []*hcl.ScriptJob{
					{
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
			name: "command options",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
						  [
							["echo", "hello", {
								cloud_sync_deployment = false
								cloud_sync_terraform_plan = "plan_a"
							}],
						  ]
						`),
					},
					{
						Command: makeCommand(t, `
							["echo", "hello", {
								cloud_sync_deployment = true
								cloud_sync_terraform_plan = "plan_b"
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
			name: "invalid command option",
			script: hcl.Script{
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
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
				Labels: labels,
				Description: hcl.NewScriptDescription(
					makeAttribute(t, "description", `"some description"`)),
				Jobs: []*hcl.ScriptJob{
					{
						Commands: makeCommands(t, `
							  [
								["echo", "hello", {
									cloud_sync_deployment = true
									cloud_sync_terraform_plan = "plan_a"
								}],
							  ]
							`),
					},
					{
						Command: makeCommand(t, `
								["echo", "hello", {
									cloud_sync_deployment = true
									cloud_sync_terraform_plan = "plan_a"
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
