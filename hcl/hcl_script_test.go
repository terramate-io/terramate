// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclutils"
)

func TestHCLScript(t *testing.T) {
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
		attr := makeAttribute(t, "command", expr)
		parsed := hcl.Command(*attr)
		return &parsed
	}

	makeCommands := func(t *testing.T, expr string) *hcl.Commands {
		attr := makeAttribute(t, "commands", expr)
		parsed := hcl.Commands(*attr)
		return &parsed
	}

	for _, tc := range []testcase{
		{
			name: "script block should not be parsed when feature not enabled",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  command = ["echo", "hello"]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("script.tm", Start(2, 8, 8), End(2, 33, 33))),
				},
			},
		},
		{
			name: "script with unrecognized blocks",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						  description = "some desc"
						  block1 {}
						  block2 {}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptUnrecognizedBlock,
						Mkrange("script.tm", Start(4, 9, 78), End(4, 15, 84))),
					errors.E(hcl.ErrScriptUnrecognizedBlock,
						Mkrange("script.tm", Start(5, 9, 96), End(5, 15, 102))),
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(6, 9, 114))),
				},
			},
		},
		{
			name: "script without a description attr",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						job {
							command = ["echo", "hello"]
						}
					  }
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Experiments: []string{"scripts"},
						},
					},
					Scripts: []*hcl.Script{
						{
							Labels: []string{"group1", "script1"},
							Jobs: []*hcl.ScriptJob{
								{
									Command: makeCommand(t, `["echo", "hello"]`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "script without a label",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script {
						description = "some description"
						job {
						  command = ["echo", "hello"]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptNoLabels,
						Mkrange("script.tm", Start(2, 8, 8), End(2, 14, 14))),
				},
			},
		},
		{
			name: "multiple scripts with same labels",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "a" "b" {
						description = "some description"
						job {
						  command = ["echo", "hello"]
						}
					  }
					  script "a" "b" {
						description = "other description"
						job {
						  command = ["echo", "other command"]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptRedeclared,
						Mkrange("script.tm", Start(8, 8, 136), End(8, 22, 150))),
				},
			},
		},
		{
			name: "script with a description attr",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(4, 9, 83))),
				},
			},
		},
		{
			name: "script with an unrecognized attr",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						unknownattr = "abc"
						job {
						  command = ["ls"]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptUnrecognizedAttr,
						Mkrange("script.tm", Start(4, 7, 81), End(4, 18, 92))),
				},
			},
		},
		{
			name: "script with a description attr and job command",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  command = ["echo", "hello"]
						}
					  }
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Experiments: []string{"scripts"},
						},
					},
					Scripts: []*hcl.Script{
						{
							Labels:      []string{"group1", "script1"},
							Description: makeAttribute(t, "description", `"some description"`),
							Jobs: []*hcl.ScriptJob{
								{
									Command: makeCommand(t, `["echo", "hello"]`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "script with an unrecognized child block of job",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  command = ["echo", "hello"]
						  someblock {}
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptUnrecognizedBlock,
						Mkrange("script.tm", Start(6, 9, 131), End(6, 18, 140))),
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(8, 9, 160))),
				},
			},
		},
		{
			name: "script with a description attr and job commands",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  commands = [
							["echo", "hello"],
							["echo", "bye"],
						  ]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{},
				config: hcl.Config{
					Scripts: []*hcl.Script{
						{
							Labels:      []string{"group1", "script1"},
							Description: makeAttribute(t, "description", `"some description"`),
							Jobs: []*hcl.ScriptJob{
								{
									Commands: makeCommands(t, `[["echo", "hello"], ["echo", "bye"]]`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "script with job command and commands",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  command = ["ls", "-l"]
						  commands = [
							["echo", "hello"],
							["echo", "bye"],
						  ]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptCmdConflict,
						Mkrange("script.tm", Start(5, 9, 95), End(5, 16, 102))),
					errors.E(hcl.ErrScriptCmdConflict,
						Mkrange("script.tm", Start(6, 9, 126), End(6, 17, 134))),
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(11, 9, 215))),
				},
			},
		},
		{
			name: "script with no job commands",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptNoCmds,
						Mkrange("script.tm", Start(4, 7, 81), End(5, 8, 94))),
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(6, 9, 103))),
				},
			},
		},
		{
			name: "script with unrecognized job attrs",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  command = ["ls", "-l"]
						  unknownattr = "abc"
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrScriptJobUnrecognizedAttr,
						Mkrange("script.tm", Start(6, 9, 126), End(6, 20, 137))),
					errors.E(hcl.ErrScriptMissingOrInvalidJob,
						Mkrange("script.tm", Start(2, 8, 8), End(8, 9, 162))),
				},
			},
		},
		{
			name: "script with multiple jobs",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  commands = [
							["echo", "hello"],
							["echo", "bye"],
						  ]
						}
						job {
						  commands = [
							["ls", "-l"],
							["date"],
						  ]
						}
						job {
						  command = ["stat", "."]
						}
					  }
					`,
				},
			},
			want: want{
				errs: []error{},
				config: hcl.Config{
					Scripts: []*hcl.Script{
						{
							Labels:      []string{"group1", "script1"},
							Description: makeAttribute(t, "description", `"some description"`),
							Jobs: []*hcl.ScriptJob{
								{
									Commands: makeCommands(t, `[["echo", "hello"], ["echo", "bye"]]`),
								},
								{
									Commands: makeCommands(t, `[["ls", "-l"], ["date"]]`),
								},
								{
									Command: makeCommand(t, `["stat", "."]`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple scripts",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "script.tm",
					body: `
					  script "group1" "script1" {
						description = "script1 desc"
						job {
						  commands = [
							["echo", "hello"],
							["echo", "bye"],
						  ]
						}
					  }

					  script "group1" "script2" {
						description = "script2 desc"
						job {
						  commands = [
							["cat", "main.tf"],
						  ]
						}
					  }
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Experiments: []string{"scripts"},
						},
					},
					Scripts: []*hcl.Script{
						{
							Labels:      []string{"group1", "script1"},
							Description: makeAttribute(t, "description", `"script1 desc"`),
							Jobs: []*hcl.ScriptJob{
								{
									Commands: makeCommands(t, `[["echo", "hello"], ["echo", "bye"]]`),
								},
							},
						},
						{
							Labels:      []string{"group1", "script2"},
							Description: makeAttribute(t, "description", `"script2 desc"`),
							Jobs: []*hcl.ScriptJob{
								{
									Commands: makeCommands(t, `[["cat", "main.tf"]]`),
								},
							},
						},
					},
				},
			},
		},
		{
			name:                  "script inside a stack dir",
			parsedir:              "stack",
			loadExperimentsConfig: true,
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
					  terramate {
						  config {
							  experiments = ["scripts"]
						  }
					  }
					`,
				},
				{
					filename: "stack/stack.tm",
					body: `
						stack {
							name = "stack"
							description = "some stack"
							after = ["after"]
							before = ["before"]
							wants = ["wants"]
							wanted_by = ["wanted"]
							watch = ["watch"]
						}
					`,
				},
				{
					filename: "stack/script.tm",
					body: `
					  script "group1" "script1" {
						description = "some description"
						job {
						  name = "hello job"
						  description = "hello job description"
						  command = ["echo", "hello"]
						}
					  }
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Name:        "stack",
						Description: "some stack",
						After:       []string{"after"},
						Before:      []string{"before"},
						Wants:       []string{"wants"},
						WantedBy:    []string{"wanted"},
						Watch:       []string{"watch"},
					},
					Scripts: []*hcl.Script{
						{
							Labels:      []string{"group1", "script1"},
							Description: makeAttribute(t, "description", `"some description"`),
							Jobs: []*hcl.ScriptJob{
								{
									Name:        makeAttribute(t, "name", `"hello job"`),
									Description: makeAttribute(t, "description", `"hello job description"`),
									Command:     makeCommand(t, `["echo", "hello"]`),
								},
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
