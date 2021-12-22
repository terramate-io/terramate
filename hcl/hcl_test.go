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
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
)

type want struct {
	err    error
	config hcl.Config
}
type testcase struct {
	name  string
	input string
	want  want
}

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
			name: "empty config",
			want: want{
				err: hcl.ErrNoTerramateBlock,
			},
		},
		{
			name: "required_version > 0.0.0",
			input: `
	terramate {
	       required_version = "> 0.0.0"
	}
	`,
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
			input: `
terramate {
	backend "something" {
		something = "something else"
	}
}
	`,
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
			input: `
terramate {
	backend "ah" {}
	backend "something" {
		something = "something else"
	}
}
	`,
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "backend with nested blocks",
			input: `
terramate {
	backend "my-label" {
		something = "something else"
		other {
			test = 1
		}
	}
}
`,
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
			input: `
terramate {
	backend {
		something = "something else"
	}
}
	`,
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "backend with more than 1 label - fails",
			input: `
terramate {
	backend "1" "2" {
		something = "something else"
	}
}
	`,
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "empty backend",
			input: `
	terramate {
		   backend "something" {
		   }
	}
	`,
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
			input: `
terramate {

}
`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
				},
			},
		},
		{
			name: "empty config block returns empty config",
			input: `
terramate {
	config {}
}
`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
				},
			},
		},
		{
			name: "unrecognized config attribute",
			input: `
terramate {
	config {
		something = "bleh"
	}
}
`,
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "empty config.git block",
			input: `
terramate {
	config {
		git {}
	}
}
`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
				},
			},
		},
		{
			name: "basic config.git block",
			input: `
terramate {
	config {
		git {
			branch = "trunk"
		}
	}
}
`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: hcl.RootConfig{
							Git: hcl.GitConfig{
								Branch: "trunk",
							},
						},
					},
				},
			},
		},
		{
			name: "all fields set for config.git",
			input: `
terramate {
	config {
		git {
			branch = "trunk"
			remote = "upstream"
			baseRef = "upstream/trunk"
			defaultBranchBaseRef = "HEAD~2"
		}
	}
}
`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: hcl.RootConfig{
							Git: hcl.GitConfig{
								Branch:               "trunk",
								Remote:               "upstream",
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

func TestHCLParserAfter(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "after: empty set works",
			input: `
terramate {
	required_version = ""
}
stack {}`,
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "'after' single entry",
			input: `
terramate {
	required_version = ""
}

stack {
	after = ["test"]
}`,
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
			input: `
terramate {
	required_version = ""
}

stack {
	after = [1]
}`,
			want: want{
				err: hcl.ErrStackInvalidRunOrder,
			},
		},
		{
			name: "'after' duplicated entry",
			input: `
terramate {
	required_version = ""
}

stack {
	after = ["test", "test"]
}`,
			want: want{
				err: hcl.ErrStackInvalidRunOrder,
			},
		},
		{
			name: "multiple 'after' fields",
			input: `
terramate {
	required_version = ""
}

stack {
	after = ["test"]
	after = []
}`,
			want: want{
				err: hcl.ErrHCLSyntax,
			},
		},
	} {
		testParser(t, tc)
	}
}

func testParser(t *testing.T, tc testcase) {
	t.Run(tc.name, func(t *testing.T) {
		got, err := hcl.Parse(tc.name, []byte(tc.input))
		assert.IsError(t, err, tc.want.err)

		if tc.want.err == nil {
			test.AssertTerramateConfig(t, *got, tc.want.config)
		}
	})
}
