// Copyright 2021 Mineiros GmbH
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

package hcl_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/rs/zerolog"
)

type (
	want struct {
		err    error
		config hcl.Config
	}

	cfgfile struct {
		filename string
		body     string
	}

	testcase struct {
		name  string
		input []cfgfile
		want  want
	}
)

func TestHCLParserModules(t *testing.T) {
	type want struct {
		modules []hcl.Module
		err     error
	}
	type testcase struct {
		name  string
		input string
		want  want
	}

	for _, tc := range []testcase{
		{
			name:  "module must have 1 label",
			input: `module {}`,
			want: want{
				err: hcl.ErrMalformedTerraform,
			},
		},
		{
			name:  "module must have a source attribute",
			input: `module "test" {}`,
			want: want{
				err: hcl.ErrMalformedTerraform,
			},
		},
		{
			name:  "empty source is a valid module",
			input: `module "test" {source = ""}`,
			want: want{
				modules: []hcl.Module{
					{
						Source: "",
					},
				},
			},
		},
		{
			name:  "valid module",
			input: `module "test" {source = "test"}`,
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
				},
			},
		},
		{
			name: "mixing modules and attributes, ignore attrs",
			input: `
				a = 1
				module "test" {
					source = "test"
				}
				b = 1
			`,
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
				},
			},
		},
		{
			name: "multiple modules",
			input: `
a = 1
module "test" {
	source = "test"
}
b = 1
module "bleh" {
	source = "bleh"
}
`,
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
					{
						Source: "bleh",
					},
				},
			},
		},
		{
			name: "ignore if source is not a string",
			input: `
module "test" {
	source = -1
}
`,
			want: want{
				err: hcl.ErrMalformedTerraform,
			},
		},
		{
			name:  "variable interpolation in the source string - fails",
			input: "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			want: want{
				err: hcl.ErrMalformedTerraform,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := test.WriteFile(t, "", "main.tf", tc.input)

			modules, err := hcl.ParseModules(path)
			assert.IsError(t, err, tc.want.err)

			assert.EqualInts(t, len(tc.want.modules), len(modules), "modules len mismatch")

			for i := 0; i < len(tc.want.modules); i++ {
				assert.EqualStrings(t, tc.want.modules[i].Source, modules[i].Source,
					"module source mismatch")
			}
		})
	}
}

func TestHCLParserTerramateBlock(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "unrecognized block",
			input: []cfgfile{
				{
					body: `something {}`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "unrecognized attribute",
			input: []cfgfile{
				{
					body: `
						terramate{}
						something = 1
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "unrecognized attribute inside terramate block",
			input: []cfgfile{
				{
					body: `
						terramate{
							something = 1
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "unrecognized block",
			input: []cfgfile{
				{
					body: `terramate{
							something {}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "multiple empty terramate blocks on same file",
			input: []cfgfile{
				{
					body: `
						terramate{}
						terramate{}
					`,
				},
			},
			want: want{
				config: hcl.Config{Terramate: &hcl.Terramate{}},
			},
		},
		{
			name: "empty config",
			want: want{
				config: hcl.Config{},
			},
		},
		{
			name: "invalid version",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = 1
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "interpolation not allowed at req_version",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = "${test.version}"
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "invalid attribute",
			input: []cfgfile{
				{
					body: `
						terramate {
							version = 1
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "required_version > 0.0.0",
			input: []cfgfile{
				{
					body: `
						terramate {
						       required_version = "> 0.0.0"
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "> 0.0.0",
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserBackend(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "backend with attributes",
			input: []cfgfile{
				{
					body: `
						terramate {
							backend "something" {
								something = "something else"
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Backend: &hclsyntax.Block{
							Type:   "backend",
							Labels: []string{"something"},
						},
					},
				},
			},
		},
		{
			name: "multiple backend blocks - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							backend "ah" {}
							backend "something" {
								something = "something else"
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "backend with nested blocks",
			input: []cfgfile{
				{
					body: `
						terramate {
							backend "my-label" {
								something = "something else"
								other {
									test = 1
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Backend: &hclsyntax.Block{
							Type:   "backend",
							Labels: []string{"my-label"},
						},
					},
				},
			},
		},
		{
			name: "backend with no labels - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							backend {
								something = "something else"
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "backend with more than 1 label - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							backend "1" "2" {
								something = "something else"
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "empty backend",
			input: []cfgfile{
				{
					body: `
						terramate {
							   backend "something" {
							   }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Backend: &hclsyntax.Block{
							Type:   "backend",
							Labels: []string{"something"},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserRootConfig(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "no config returns empty config",
			input: []cfgfile{
				{
					body: `terramate {}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
				},
			},
		},
		{
			name: "empty config block returns empty config",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{},
					},
				},
			},
		},
		{
			name: "unrecognized config attribute",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {
								something = "bleh"
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "empty config.git block",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {
								git {}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{},
						},
					},
				},
			},
		},
		{
			name: "multiple empty config blocks",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {}
							config {}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{},
					},
				},
			},
		},
		{
			name: "multiple config.generate blocks",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {}
						    generate {}
						  }
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "config.generate block with unknown attribute",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {
						      very_unknown_attribute = "oopsie"
						    }
						  }
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "basic config.git block",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
							},
						},
					},
				},
			},
		},
		{
			name: "empty config.generate block",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Generate: &hcl.GenerateConfig{},
						},
					},
				},
			},
		},
		{
			name: "full config.generate block",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {
						      backend_config_filename = "backend.tf"
						      locals_filename = "locals.tf"
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Generate: &hcl.GenerateConfig{
								BackendCfgFilename: "backend.tf",
								LocalsFilename:     "locals.tf",
							},
						},
					},
				},
			},
		},
		{
			name: "config.generate with conflicting config fails",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {
						      backend_config_filename = "file.tf"
						      locals_filename = "file.tf"
						    }
						  }
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "config.generate block with invalid cfg",
			input: []cfgfile{
				{
					body: `
						terramate {
						  config {
						    generate {
						      backend_config_filename = true
						      locals_filename = 666
						    }
						  }
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "all fields set for config.git",
			input: []cfgfile{
				{
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
									default_remote = "upstream"
									base_ref = "upstream/trunk"
									default_branch_base_ref = "HEAD~2"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:        "trunk",
								DefaultRemote:        "upstream",
								BaseRef:              "upstream/trunk",
								DefaultBranchBaseRef: "HEAD~2",
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserStack(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "empty stack block",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "multiple stack blocks",
			input: []cfgfile{
				{
					body: `
						terramate {}
						stack{}
						stack{}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "empty name",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = ""
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "name is not a string - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = 1
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "name has interpolation - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = "${test}"
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "unrecognized attribute name - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {
							bleh = "a"
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "after: empty set works",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "'after' single entry",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						After: []string{"test"},
					},
				},
			},
		},
		{
			name: "'after' invalid element entry",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = [1]
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrStackInvalidRunOrder,
			},
		},
		{
			name: "'after' duplicated entry",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["test", "test"]
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrStackInvalidRunOrder,
			},
		},
		{
			name: "multiple 'after' fields - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["test"]
							after = []
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrHCLSyntax,
			},
		},
		{
			name: "multiple 'before' fields - fails",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = []
							before = []
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrHCLSyntax,
			},
		},
		{
			name: "'before' single entry",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
					},
				},
			},
		},
		{
			name: "'before' multiple entries",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something", "something-else", "test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something", "something-else", "test"},
					},
				},
			},
		},
		{
			name: "stack with valid description",
			input: []cfgfile{
				{
					body: `
						stack {
							description = "some cool description"
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "some cool description",
					},
				},
			},
		},
		{
			name: "stack with multiline description",
			input: []cfgfile{
				{
					body: `
					stack {
						description =  <<-EOD
	line1
	line2
	EOD
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "line1\nline2",
					},
				},
			},
		},
		{
			name: "'before' and 'after'",
			input: []cfgfile{
				{
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
							after = ["else"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
						After:  []string{"else"},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}

func TestHCLParserTerramateBlocksMerging(t *testing.T) {
	tcases := []testcase{
		{
			name: "two config file with terramate blocks",
			input: []cfgfile{
				{
					filename: "version.tm",
					body: `
						terramate {
							required_version = "0.0.1"
						}
					`,
				},
				{
					filename: "config.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "0.0.1",
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
							},
						},
					},
				},
			},
		},
		{
			name: "terramate.generate and terramate.config on different files",
			input: []cfgfile{
				{
					filename: "generate.tm.hcl",
					body: `
						terramate {
							config {
								generate {
									locals_filename = "locals.tf"
									backend_config_filename = "backend.tf"
								}
							}
						}
					`,
				},
				{
					filename: "git.tm.hcl",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
									default_remote = "upstream"
									base_ref = "upstream/trunk"
									default_branch_base_ref = "HEAD~2"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								BaseRef:              "upstream/trunk",
								DefaultBranch:        "trunk",
								DefaultRemote:        "upstream",
								DefaultBranchBaseRef: "HEAD~2",
							},
							Generate: &hcl.GenerateConfig{
								LocalsFilename:     "locals.tf",
								BackendCfgFilename: "backend.tf",
							},
						},
					},
				},
			},
		},
		{
			name: "different terramate.generate and terramate.config on same file",
			input: []cfgfile{
				{
					filename: "config.tm",
					body: `
						terramate {
							config {
								generate {
									locals_filename = "locals.tf"
									backend_config_filename = "backend.tf"
								}
							}
						}
						terramate {
							config {
								git {
									default_branch = "trunk"
									default_remote = "upstream"
									base_ref = "upstream/trunk"
									default_branch_base_ref = "HEAD~2"
								}
							}
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								BaseRef:              "upstream/trunk",
								DefaultBranch:        "trunk",
								DefaultRemote:        "upstream",
								DefaultBranchBaseRef: "HEAD~2",
							},
							Generate: &hcl.GenerateConfig{
								LocalsFilename:     "locals.tf",
								BackendCfgFilename: "backend.tf",
							},
						},
					},
				},
			},
		},
		{
			name: "three config files with terramate and stack blocks",
			input: []cfgfile{
				{
					filename: "version.tm",
					body: `
						terramate {
							required_version = "6.6.6"
						}
					`,
				},
				{
					filename: "config.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
				{
					filename: "stack.tm",
					body: `
						stack {
							name = "stack"
							description = "some stack"
							after = ["after"]
							before = ["before"]
							wants = ["wants"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "6.6.6",
						RootConfig: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch: "trunk",
							},
						},
					},
					Stack: &hcl.Stack{
						Name:        "stack",
						Description: "some stack",
						After:       []string{"after"},
						Before:      []string{"before"},
						Wants:       []string{"wants"},
					},
				},
			},
		},
		{
			name: "multiple files with stack blocks fail",
			input: []cfgfile{
				{
					filename: "stack_name.tm",
					body: `
						stack {
							name = "stack"
						}
					`,
				},
				{
					filename: "stack_desc.tm",
					body: `
						stack {
							description = "some stack"
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "multiple files with terramate.config.git blocks fail",
			input: []cfgfile{
				{
					filename: "git.tm",
					body: `
						terramate {
							config {
								git {
									default_branch = "trunk"
								}
							}
						}
					`,
				},
				{
					filename: "gitagain.tm",
					body: `
						terramate {
							config {
								git {
									base_ref = "trunk"
								}
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "multiple files with terramate.config.generate blocks fail",
			input: []cfgfile{
				{
					filename: "locals.tm",
					body: `
						terramate {
							config {
								generate {
									locals_filename = "test.tf"
								}
							}
						}
					`,
				},
				{
					filename: "backend.tm",
					body: `
						terramate {
							config {
								generate {
									backend_config_filename = "backend.tf"
								}
							}
						}
					`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
	}

	for _, tc := range tcases {
		testParser(t, tc)
	}
}

func testParser(t *testing.T, tc testcase) {
	t.Run(tc.name, func(t *testing.T) {
		configsDir := t.TempDir()
		for _, inputConfigFile := range tc.input {
			filename := inputConfigFile.filename
			if filename == "" {
				filename = config.DefaultFilename
			}
			test.WriteFile(t, configsDir, filename, inputConfigFile.body)
		}
		got, err := hcl.ParseDir(configsDir)
		assert.IsError(t, err, tc.want.err)

		if tc.want.err == nil {
			test.AssertTerramateConfig(t, got, tc.want.config)
		}
	})

	// This is a lazy way to piggyback on our current set of tests
	// to test that we have identical behavior when configuration
	// is on a different file that is not Terramate default.
	// Old tests don't inform a specific filename (assuming default).
	// We use this to test each of these simple scenarios with
	// different filenames (other than default).
	if len(tc.input) != 1 || tc.input[0].filename != "" {
		return
	}

	validConfigFilenames := []string{
		"config.tm",
		"config.tm.hcl",
	}

	for _, filename := range validConfigFilenames {
		newtc := testcase{
			name: fmt.Sprintf("%s with filename %s", tc.name, filename),
			input: []cfgfile{
				{
					filename: filename,
					body:     tc.input[0].body,
				},
			},
			want: tc.want,
		}
		testParser(t, newtc)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
