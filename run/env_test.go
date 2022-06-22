package run_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/run"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/errors"
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
			wantErr error
		}
	)

	expr := hclwrite.Expression
	terramate := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("terramate", builders...)
	}
	config := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("config", builders...)
	}
	runblock := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("run", builders...)
	}
	env := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("env", builders...)
	}
	runEnvCfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return terramate(config(runblock(env(builders...))))
	}

	tcases := []testcase{
		{
			name: "single stack with env loaded from host env",
			hostenv: run.EnvVars{
				"TESTING_RUN_ENV_VAR": "666",
			},
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: runEnvCfg(
						expr("test", "env.TESTING_RUN_ENV_VAR"),
					),
				},
			},
			want: map[string]result{
				"stack": {
					env: run.EnvVars{
						"test": "666",
					},
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
				test.AppendFile(t, path, "run_env_test_cfg.hcl", cfg.add.String())
			}

			for stackRelPath, wantres := range tcase.want {
				stack := s.LoadStack(filepath.Join(s.RootDir(), stackRelPath))
				gotvars, err := run.Env(s.RootDir(), stack)

				errors.Assert(t, err, wantres.err)
				test.AssertDiff(t, gotvars, wantres.env)
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
