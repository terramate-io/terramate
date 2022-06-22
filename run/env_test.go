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
			hostenv run.EnvVars
			layout  []string
			configs []hclconfig
			want    map[string]result
		}
	)

	expr := hclwrite.Expression
	str := hclwrite.String
	hcldoc := hclwrite.BuildHCL
	block := hclwrite.BuildBlock
	terramate := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("terramate", builders...)
	}
	config := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("config", builders...)
	}
	runblock := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("run", builders...)
	}
	env := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("env", builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return block("globals", builders...)
	}
	runEnvCfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return terramate(config(runblock(env(builders...))))
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
			hostenv: run.EnvVars{
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
						expr("testenv", "env.TESTING_RUN_ENV_VAR"),
						str("teststr", "plain string"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"testenv": "666",
						"teststr": "plain string",
					},
				},
				"stacks/stack-2": {
					env: run.EnvVars{
						"testenv": "666",
						"teststr": "plain string",
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
						expr("env1", "global.env"),
						expr("env2", "terramate.stack.name"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: globals(
						str("env", "stack-1 global"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: globals(
						str("env", "stack-2 global"),
					),
				},
			},
			want: map[string]result{
				"stacks/stack-1": {
					env: run.EnvVars{
						"env1": "stack-1 global",
						"env2": "stack-1",
					},
				},
				"stacks/stack-2": {
					env: run.EnvVars{
						"env1": "stack-2 global",
						"env2": "stack-2",
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
					add: hcldoc(
						block("notvalidterramate"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					err: errors.E(run.ErrParsingCfg),
				},
			},
		},
	}

	for _, tcase := range tcases {

		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, "run_env_test_cfg.tm", cfg.add.String())
			}

			for name, value := range tcase.hostenv {
				t.Setenv(name, value)
			}

			for stackRelPath, wantres := range tcase.want {
				stack := s.LoadStack(filepath.Join(s.RootDir(), stackRelPath))
				gotvars, err := run.Env(s.RootDir(), stack)

				errorstest.Assert(t, err, wantres.err)
				test.AssertDiff(t, gotvars, wantres.env)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
