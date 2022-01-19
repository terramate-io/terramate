package generate_test

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/hclwrite"
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
		hclblock struct {
			path string
			cfg  fmt.Stringer
		}

		want struct {
			cfg generate.StackCfg
			err error
		}

		testcase struct {
			name    string
			stack   string
			configs []hclblock
			want    want
		}
	)

	// gen instead of generate because name conflicts with generate pkg
	gen := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate", builders...)
	}
	// avoid conflicts with config package, so define on smaller scope
	config := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
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
			configs: []hclblock{
				{
					path: "/stack",
					cfg: hcldoc(
						terramate(
							config(
								gen(
									str("backend_config_filename", "backend.tf"),
									str("locals_filename", "locals.tf"),
								),
							),
						),
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

		})
	}
}
