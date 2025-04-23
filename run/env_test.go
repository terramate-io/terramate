// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/test"
	errorstest "github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestLoadRunEnv(t *testing.T) {
	type (
		hclconfig struct {
			path  string
			fname string
			add   fmt.Stringer
		}
		result struct {
			env    run.EnvVars
			enverr error
		}
		testcase struct {
			name    string
			hostenv map[string]string
			layout  []string
			configs []hclconfig
			want    map[string]result
		}
	)

	runEnvCfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return Terramate(Config(Run(Env(builders...))))
	}

	tcases := []testcase{
		{
			name: "no env config",
			layout: []string{
				"s:stack",
			},
		},
		{
			name: "stacks with env loaded from host env and literals",
			hostenv: map[string]string{
				"TESTING_RUN_ENV_VAR": "666",
			},
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("testenv", "env.TESTING_RUN_ENV_VAR"),
						Str("teststr", "plain string"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"testenv=666",
						"teststr=plain string",
					},
				},
				"stacks/stack-2": {
					env: run.EnvVars{
						"testenv=666",
						"teststr=plain string",
					},
				},
			},
		},
		{
			// GH issue: https://github.com/terramate-io/terramate/issues/1710
			name: "regression test: incomplete env due to equal sign split",
			hostenv: map[string]string{
				"TESTING_RUN_ENV_VAR": "A=B=C",
			},
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("testenv", "env.TESTING_RUN_ENV_VAR"),
						Str("teststr", "plain=with=equal=string"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"testenv=A=B=C",
						`teststr=plain=with=equal=string`,
					},
				},
			},
		},
		{
			name: "stacks with env loaded from globals and metadata",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("env1", "global.env"),
						Expr("env2", "terramate.stack.name"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("env", "stack-1 global"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Str("env", "stack-2 global"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"env1=stack-1 global",
						"env2=stack-1",
					},
				},
				"stacks/stack-2": {
					env: run.EnvVars{
						"env1=stack-2 global",
						"env2=stack-2",
					},
				},
			},
		},
		{
			name: "fails on globals loading failure",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("env", "global.a"),
					),
				},
				{
					path: "/stack",
					add: Globals(
						Expr("a", "undefined"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					enverr: errors.E(run.ErrLoadingGlobals),
				},
			},
		},
		{
			name: "fails evaluating undefined attribute",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("env", "something.undefined"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					enverr: errors.E(run.ErrEval),
				},
			},
		},
		{
			name: "fails if attribute is not string",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("env", "[]"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					enverr: errors.E(run.ErrInvalidEnvVarType),
				},
			},
		},
		{
			name: "dirs can override root env",
			hostenv: map[string]string{
				"TESTING_RUN_ENV_VAR": "666",
			},
			layout: []string{
				"s:dir/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("testenv", "env.TESTING_RUN_ENV_VAR"),
						Str("teststr", "plain string"),
					),
				},
				{
					path: "/dir",
					add: runEnvCfg(
						Str("testenv", "overridden"),
					),
				},
			},
			want: map[string]result{
				"dir/stack-1": {
					env: run.EnvVars{
						"testenv=overridden",
						"teststr=plain string",
					},
				},
			},
		},
		{
			name: "dirs can override other dirs",
			layout: []string{
				"s:dir1/dir2/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/dir1",
					add: runEnvCfg(
						Str("teststr", "defined at /dir1"),
					),
				},
				{
					path: "/dir1/dir2",
					add: runEnvCfg(
						Str("teststr", "defined at /dir1/dir2"),
					),
				},
			},
			want: map[string]result{
				"dir1/dir2/stack-1": {
					env: run.EnvVars{
						"teststr=defined at /dir1/dir2",
					},
				},
			},
		},
		{
			name: "stacks can override root env",
			hostenv: map[string]string{
				"TESTING_RUN_ENV_VAR": "666",
			},
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						Expr("testenv", "env.TESTING_RUN_ENV_VAR"),
						Str("teststr", "plain string"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Str("testenv", "overridden"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"testenv=overridden",
						"teststr=plain string",
					},
				},
				"stacks/stack-2": {
					env: run.EnvVars{
						"testenv=666",
						"teststr=plain string",
					},
				},
			},
		},
		{
			name: "stacks can override parent stacks",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1"),
					),
				},
				{
					path: "/stacks/stack-1/stack-2",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1/stack-2"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1",
					},
				},
				"stacks/stack-1/stack-2": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1/stack-2",
					},
				},
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
		{
			name: "unset on lower levels",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1"),
					),
				},
				{
					path: "/stacks/stack-1/stack-2",
					add: runEnvCfg(
						Expr("teststr", "unset"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1",
					},
				},
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
		{
			name: "unset on lower levels aaa",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1"),
					),
				},
				{
					path: "/stacks/stack-1/stack-2",
					add: runEnvCfg(
						Expr("teststr", "unset"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1",
					},
				},
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
		{
			name: "unset on middle level dirs and re-assigning in lower level",
			layout: []string{
				"s:stacks/dir/stack-1",
				"s:stacks/dir/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/dir",
					add: runEnvCfg(
						Expr("teststr", "unset"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
		{
			name: "unset on middle level stacks and re-assigning in lower level",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Expr("teststr", "unset"),
					),
				},
				{
					path: "/stacks/stack-1/stack-2",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1/stack-2"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1/stack-2": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1/stack-2",
					},
				},
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
		{
			name: "set null ignores the env",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: runEnvCfg(
						Expr("teststr", "null"),
					),
				},
			},
		},
		{
			name: "null on lower levels",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-1/stack-2",
				"s:stacks/stack-3",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: runEnvCfg(
						Str("teststr", "/stacks"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: runEnvCfg(
						Str("teststr", "/stacks/stack-1"),
					),
				},
				{
					path: "/stacks/stack-1/stack-2",
					add: runEnvCfg(
						Expr("teststr", "null"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"teststr=/stacks/stack-1",
					},
				},
				"stacks/stack-3": {
					env: run.EnvVars{
						"teststr=/stacks",
					},
				},
			},
		},
	}

	// TODO(i4k): these tests should not call setenv()!
	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tcase.layout)
			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				fname := cfg.fname
				if fname == "" {
					fname = "run_env_test_cfg.tm"
				}
				test.AppendFile(t, path, fname, cfg.add.String())
			}

			for name, value := range tcase.hostenv {
				t.Setenv(name, value)
			}

			root, err := config.LoadRoot(s.RootDir(), false)
			if err != nil {
				t.Fatal(err)
			}

			for _, stackPath := range root.Stacks() {
				stack, err := config.LoadStack(root, stackPath)
				assert.NoError(t, err)

				wantres := tcase.want[stackPath.String()[1:]]

				gotvars, err := run.LoadEnv(root, stack)
				errorstest.Assert(t, err, wantres.enverr)
				if err != nil {
					continue
				}
				t.Logf("stack: %v, vars: %v", stackPath, gotvars)
				test.AssertDiff(t, gotvars, wantres.env)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
