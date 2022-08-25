// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package run_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/run"
	"github.com/mineiros-io/terramate/test"
	errorstest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadRunEnv(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		result struct {
			env run.EnvVars
			err error
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
					err: errors.E(run.ErrParsingCfg),
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
					err: errors.E(run.ErrLoadingGlobals),
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
					err: errors.E(run.ErrEval),
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
					err: errors.E(run.ErrInvalidEnvVarType),
				},
			},
		},
	}

	for _, tcase := range tcases {

		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			projmeta := s.LoadProjectMetadata()

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, "run_env_test_cfg.tm", cfg.add.String())
			}

			for name, value := range tcase.hostenv {
				t.Setenv(name, value)
			}

			for stackRelPath, wantres := range tcase.want {
				stack := s.LoadStack(stackRelPath)
				gotvars, err := run.LoadEnv(projmeta, stack)

				errorstest.Assert(t, err, wantres.err)
				test.AssertDiff(t, gotvars, wantres.env)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
