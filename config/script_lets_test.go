// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/zclconf/go-cty/cty"
)

func TestScriptLetsEval(t *testing.T) {
	t.Parallel()

	labels := []string{"some", "label"}

	for _, tc := range []scriptTestcase{
		{
			name: "script with unused lets",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Str("test", "some unused string"),
				),
				Block("job",
					Expr("commands", `[["echo", "hello"]]`),
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
							},
						},
					},
				},
			},
		},
		{
			name: "script with simple lets",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Str("test", "some string"),
				),
				Block("job",
					Expr("commands", `[["echo", let.test]]`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "some string"},
							},
						},
					},
				},
			},
		},
		{
			name: "script with lets referencing terramate.*",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("command", `[["ls", "-lha", terramate.stack.path.absolute]]`),
				),
				Block("job",
					Expr("commands", `let.command`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"ls", "-lha", "/"},
							},
						},
					},
				},
			},
		},
		{
			name: "script with lets referencing global.*",
			globals: map[string]cty.Value{
				"path": cty.StringVal("/test.txt"),
			},
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("command", `[["ls", "-lha", global.path]]`),
				),
				Block("job",
					Expr("commands", `let.command`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"ls", "-lha", "/test.txt"},
							},
						},
					},
				},
			},
		},
		{
			name: "script with command lets",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("cmd", `["echo", "hello"]`),
				),
				Block("job",
					Expr("command", `let.cmd`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Description: "some description",
				Jobs: []config.ScriptJob{
					{
						Cmd: &config.ScriptCmd{
							Args: []string{"echo", "hello"},
						},
					},
				},
			},
		},
		{
			name: "script with commands lets",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("cmd", `[
						["echo", "hello"],
						["echo", "world"],
					]`),
				),
				Block("job",
					Expr("commands", `let.cmd`),
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
							},
							{
								Args: []string{"echo", "world"},
							},
						},
					},
				},
			},
		},
		{
			name: "script with multiple lets",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("cmd1", `[["echo", "hello"]]`),
					Expr("cmd2", `[["echo", "world"]]`),
					Expr("cmds", `tm_concat(let.cmd1, let.cmd2)`),
				),
				Block("job",
					Expr("commands", `let.cmds`),
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
							},
							{
								Args: []string{"echo", "world"},
							},
						},
					},
				},
			},
		},
		{
			name: "script with multiple lets blocks are merged",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("cmd1", `[["echo", "hello"]]`),
				),
				Lets(
					Expr("cmd2", `[["echo", "world"]]`),
				),
				Lets(
					Expr("cmds", `tm_concat(let.cmd1, let.cmd2)`),
				),
				Block("job",
					Expr("commands", `let.cmds`),
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
							},
							{
								Args: []string{"echo", "world"},
							},
						},
					},
				},
			},
		},
		{
			name: "lets variables have implicit order",
			config: Script(
				Labels(labels...),
				Str("description", "some description"),
				Lets(
					Expr("cmds", `tm_concat(let.cmd1, let.cmd2)`),
				),
				Lets(
					Expr("cmd1", `[["echo", "hello"]]`),
				),
				Lets(
					Expr("cmd2", `[["echo", "world"]]`),
				),
				Block("job",
					Expr("commands", `let.cmds`),
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
							},
							{
								Args: []string{"echo", "world"},
							},
						},
					},
				},
			},
		},
		{
			name: "lets variables can be used in script.name, script.description, script.job.name and script.job.description",
			config: Script(
				Labels(labels...),
				Expr("name", `let.name`),
				Expr("description", `let.desc`),
				Lets(
					Expr("name", `tm_upper(let.hello)`),
					Expr("desc", `tm_upper(let.name)`),
					Str("hello", "hello"),
				),
				Block("job",
					Expr("name", `let.name`),
					Expr("description", `let.desc`),
					Expr("commands", `[["echo", let.name, let.desc]]`),
				),
			),
			want: config.Script{
				Labels:      labels,
				Name:        "HELLO",
				Description: "HELLO",
				Jobs: []config.ScriptJob{
					{
						Name:        "HELLO",
						Description: "HELLO",
						Cmds: []*config.ScriptCmd{
							{
								Args: []string{"echo", "HELLO", "HELLO"},
							},
						},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testScriptEval(t, tc)
		})
	}

}

func TestScriptLetsArelocalToTheirBlocks(t *testing.T) {
	tempdir := test.TempDir(t)
	hclctx := eval.NewContext(stdlib.Functions(tempdir, []string{}))
	test.AppendFile(t, tempdir, "script.tm", Doc(
		Script(
			Labels("deploy1"),
			Lets(
				Str("A", "A"),
			),
			Block("job",
				Expr("command", `[let.A]`),
			),
		),
		Script(
			Labels("deploy2"),
			Block("job",
				Expr("command", `[let.A]`),
			),
		),
	).String())
	test.AppendFile(t, tempdir, "terramate.tm", Terramate(
		Config(
			Expr("experiments", `["scripts"]`),
		),
	).String())

	cfg, err := config.LoadRoot(tempdir)
	assert.NoError(t, err)

	rootTree, ok := cfg.Lookup(project.NewPath("/"))
	if !ok {
		panic("root tree not found")
	}
	if len(rootTree.Node.Scripts) != 2 {
		panic("test expects two scripts")
	}
	got, err := config.EvalScript(hclctx, *rootTree.Node.Scripts[0])
	assert.NoError(t, err)

	want := config.Script{
		Labels: []string{"deploy1"},
		Jobs: []config.ScriptJob{
			{
				Cmd: &config.ScriptCmd{
					Args: []string{"A"},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(info.Range{})); diff != "" {
		t.Fatalf("unexpected result\n%s", diff)
	}

	// must fail because let.A is not set
	_, err = config.EvalScript(hclctx, *rootTree.Node.Scripts[1])
	errtest.AssertIsKind(t, err, config.ErrScriptSchema)
}
