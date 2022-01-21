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
		{
			name:  "config on parent",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "parent_backend.tf",
					LocalsFilename:     "parent_locals.tf",
				},
			},
		},
		{
			name:  "config on parent overrides root",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "root_backend.tf"),
							str("locals_filename", "root_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "parent_backend.tf",
					LocalsFilename:     "parent_locals.tf",
				},
			},
		},
		{
			name:  "config on stack overrides all parent configs",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "root_backend.tf"),
							str("locals_filename", "root_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "stack_backend.tf"),
							str("locals_filename", "stack_locals.tf"),
						))),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "stack_backend.tf",
					LocalsFilename:     "stack_locals.tf",
				},
			},
		},
		{
			name:  "valid config on stack ignores parent invalid config",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("oh_no_such_invalid_much_error", "parent_backend.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "stack_backend.tf"),
							str("locals_filename", "stack_locals.tf"),
						))),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "stack_backend.tf",
					LocalsFilename:     "stack_locals.tf",
				},
			},
		},
		{
			name:  "config is overridden not merged",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						terramate(cfg(gen(
							str("locals_filename", "stack_locals.tf"),
						))),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: generate.BackendCfgFilename,
					LocalsFilename:     "stack_locals.tf",
				},
			},
		},
		{
			name:  "empty generate block sets defaults",
			stack: "stack",
			configs: []hclcfg{
				{
					path: "/stack",
					body: hcldoc(
						terramate(cfg(gen())),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: generate.BackendCfgFilename,
					LocalsFilename:     generate.LocalsFilename,
				},
			},
		},
		{
			name:  "empty generate block overrides parent config with defaults",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						terramate(cfg(gen())),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: generate.BackendCfgFilename,
					LocalsFilename:     generate.LocalsFilename,
				},
			},
		},
		{
			name:  "load parent config if stack has no terramate",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "parent_backend.tf",
					LocalsFilename:     "parent_locals.tf",
				},
			},
		},
		{
			name:  "load parent config if stack has no terramate.config.generate",
			stack: "stacks/stack",
			configs: []hclcfg{
				{
					path: "/stacks",
					body: hcldoc(
						terramate(cfg(gen(
							str("backend_config_filename", "parent_backend.tf"),
							str("locals_filename", "parent_locals.tf"),
						))),
					),
				},
				{
					path: "/stacks/stack",
					body: hcldoc(
						terramate(cfg()),
						stack(),
					),
				},
			},
			want: want{
				cfg: generate.StackCfg{
					BackendCfgFilename: "parent_backend.tf",
					LocalsFilename:     "parent_locals.tf",
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
