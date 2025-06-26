// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

type runEnvTestcase struct {
	name    string
	layout  []string
	wd      string
	args    []string
	runArgs []string
	env     []string
	want    RunExpected
}

func TestRunEnv(t *testing.T) {
	for _, tc := range []runEnvTestcase{
		{
			name: "empty run env - check sentinel env name does not exist in the HOST",
			layout: []string{
				`s:stack`,
			},
			args: []string{"FOO"},
		},
		{
			name: "empty run env - check provided host env works",
			layout: []string{
				`s:stack`,
			},
			env:  []string{"FOO=TEST"},
			args: []string{"env", "FOO"},
			want: RunExpected{
				Stdout: nljoin("/stack: TEST"),
			},
		},
		{
			name: "setting run env at root is inherited in all child stacks",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
			},
			env:  []string{"FOO=FOO"},
			args: []string{"env", "FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO", "/s1/a: BAR",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "declaring run env in child stacks overrides root declaration",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:s1/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR S1"),
							),
						),
					),
				).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"env", "FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR S1",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO", "/s1/a: BAR S1",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "declaring run env in child stacks overrides parent declaration",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:s1/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR S1"),
							),
						),
					),
				).String(),
				`f:s1/a/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR S1/A"),
							),
						),
					),
				).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"env", "FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR S1",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO", "/s1/a: BAR S1/A",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "unsetting env in child using null",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:s1/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR S1"),
							),
						),
					),
				).String(),
				`f:s1/a/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Expr("BAR", "null"),
							),
						),
					),
				).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"env", "FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR S1",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "unsetting env in child using unset",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:s1/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR S1"),
							),
						),
					),
				).String(),
				`f:s1/a/env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Expr("BAR", "unset"),
							),
						),
					),
				).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR S1",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "unsetting host env does nothing",
			layout: []string{
				`f:root.tm:` + Terramate(
					Config(
						Run(
							Env(
								Expr("FOO", "null"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
			},
			env:  []string{"FOO=FOO"},
			args: []string{"FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO",
					"/s2: FOO",
				),
			},
		},
		{
			name: "importing run env at root is inherited in all child stacks",
			layout: []string{
				`f:/modules/run_env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR"),
							),
						),
					),
				).String(),
				`f:/root_env.tm:` + Import(
					Str("source", "/modules/run_env.tm"),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
			},
			env:  []string{"FOO=FOO"},
			args: []string{"FOO", "BAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR",
					"/s2: FOO", "/s2: BAR",
					"/s1/a: FOO", "/s1/a: BAR",
					"/s2/a: FOO", "/s2/a: BAR",
				),
			},
		},
		{
			name: "importing run env at child level is taking into consideration when loading env",
			layout: []string{
				`f:/modules/run_env1.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR1", "BAR1"),
							),
						),
					),
				).String(),
				`f:/modules/run_env2.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR2", "BAR2"),
							),
						),
					),
				).String(),
				`f:/root_env.tm:` + Import(
					Str("source", "/modules/run_env1.tm"),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:/s1/a/env.tm:` + Import(
					Str("source", "/modules/run_env2.tm"),
				).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"FOO", "BAR1", "BAR2"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO", "/s1: BAR1",
					"/s2: FOO", "/s2: BAR1",
					"/s1/a: FOO", "/s1/a: BAR1", "/s1/a: BAR2",
					"/s2/a: FOO", "/s2/a: BAR1",
				),
			},
		},
		{
			name: "stack definition overrides imported definition",
			layout: []string{
				`f:/modules/run_env.tm:` + Terramate(
					Config(
						Run(
							Env(
								Str("BAR", "BAR IMPORTED"),
								Str("CAR", "CAR IMPORTED"),
							),
						),
					),
				).String(),
				`s:s1`,
				`s:s2`,
				`s:s1/a`,
				`s:s2/a`,
				`f:/s1/a/env.tm:` + Doc(
					Import(
						Str("source", "/modules/run_env.tm"),
					),
					Terramate(
						Config(
							Run(
								Env(
									Str("BAR", "BAR OVERRIDDEN"),
								),
							),
						),
					)).String(),
			},
			env:  []string{"FOO=FOO"},
			args: []string{"FOO", "BAR", "CAR"},
			want: RunExpected{
				Stdout: nljoin(
					"/s1: FOO",
					"/s2: FOO",
					"/s1/a: FOO", "/s1/a: BAR OVERRIDDEN", "/s1/a: CAR IMPORTED",
					"/s2/a: FOO",
				),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)
			tmcli := NewCLI(t, filepath.Join(s.RootDir(), tc.wd), tc.env...)
			args := []string{"run", "--quiet"}
			if len(tc.runArgs) > 0 {
				args = append(args, tc.runArgs...)
			}
			args = append(args, "--", HelperPath, "env", s.RootDir())
			args = append(args, tc.args...)
			got := tmcli.Run(args...)
			AssertRunResult(t, got, tc.want)
		})
	}
}
