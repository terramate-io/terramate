// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run_test

import (
	"fmt"
	"path"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
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
			path string
			add  fmt.Stringer
		}
		result struct {
			env    run.EnvVars
			enverr error
			cfgerr error
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
			want: map[string]result{
				"stack": {},
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
			name: "fails on invalid root config",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						Block("notvalidterramate"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					cfgerr: errors.E(hcl.ErrTerramateSchema),
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
	}

	// TODO(i4k): these tests should not call setenv()!
	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tcase.layout)
			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, "run_env_test_cfg.tm", cfg.add.String())
			}

			for name, value := range tcase.hostenv {
				t.Setenv(name, value)
			}

			for stackRelPath, wantres := range tcase.want {
				root, err := config.LoadRoot(s.RootDir())
				if wantres.cfgerr != nil {
					errorstest.Assert(t, err, wantres.cfgerr)
					return
				}

				stack, err := config.LoadStack(root, project.NewPath(path.Join("/", stackRelPath)))
				assert.NoError(t, err)

				gotvars, err := run.LoadEnv(root, stack)
				errorstest.Assert(t, err, wantres.enverr)
				test.AssertDiff(t, gotvars, wantres.env)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
