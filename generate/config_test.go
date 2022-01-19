package generate_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// Tests
// - 2 terramate.config on same file works if generate not duplicated
// - 2 terramate.config.generate on same file fails
// - parent dir config
// - root dir config
// - stack invalid config err
// - parent invalid config err
// - stack valid config + parent invalid config = works ?

func TestGenerateStackConfigLoad(t *testing.T) {

	type (
		hclcfg struct {
			path string
			body fmt.Stringer
		}

		want struct {
			cfg generate.StackCfg
			err error
		}

		testcase struct {
			name    string
			stack   string
			configs []hclcfg
			want    want
		}
	)

	// gen instead of generate because name conflicts with generate pkg
	gen := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate", builders...)
	}
	// cfg instead of config because name conflicts with config pkg
	cfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("config", builders...)
	}

	tcases := []testcase{
		{
			name:  "default config",
			stack: "stack",
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: generate.BackendCfgFilename,
					LocalsFilename:     generate.LocalsFilename,
				},
			},
		},
		{
			name:  "backend and locals config on stack",
			stack: "stack",
			configs: []hclcfg{
				{
					path: "/stack",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "backend.tf"),
							str("locals_filename", "locals.tf"),
						))),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "backend.tf",
					LocalsFilename:     "locals.tf",
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.WriteFile(t, path, config.Filename, cfg.body.String())
			}

			got, err := generate.LoadStackCfg(s.RootDir(), stack)
			assert.IsError(t, err, tcase.want.err)

			if got != tcase.want.cfg {
				t.Fatalf("got stack cfg %v; want %v", got, tcase.want.cfg)
			}
		})
	}
}
