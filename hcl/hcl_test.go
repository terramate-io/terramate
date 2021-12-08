package hcl_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
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
			name:  "ignore module type with no label",
			input: `module {}`,
		},
		{
			name:  "ignore module type with no source attribute",
			input: `module "test" {}`,
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

			parser := hcl.NewParser()
			modules, err := parser.ParseModules(path)
			assert.IsError(t, err, tc.want.err)

			assert.EqualInts(t, len(tc.want.modules), len(modules), "modules len mismatch")

			for i := 0; i < len(tc.want.modules); i++ {
				assert.EqualStrings(t, tc.want.modules[i].Source, modules[i].Source,
					"module source mismatch")
			}
		})
	}
}

func TestHHCLParserTerramateBlock(t *testing.T) {
	type want struct {
		err   error
		block hcl.Terramate
	}
	type testcase struct {
		name  string
		input string
		want  want
	}

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
				block: hcl.Terramate{
					RequiredVersion: "> 0.0.0",
				},
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
				block: hcl.Terramate{
					Backend: &hclsyntax.Block{
						Type:   "backend",
						Labels: []string{"something"},
					},
				},
			},
		},
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
				block: hcl.Terramate{
					Backend: &hclsyntax.Block{
						Type:   "backend",
						Labels: []string{"something"},
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
				err: hcl.ErrMalformedTerramateBlock,
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
				block: hcl.Terramate{
					Backend: &hclsyntax.Block{
						Type:   "backend",
						Labels: []string{"my-label"},
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
				err: hcl.ErrMalformedTerramateBlock,
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
				err: hcl.ErrMalformedTerramateBlock,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := hcl.NewParser()
			got, err := p.Parse(tc.name, []byte(tc.input))
			assert.IsError(t, err, tc.want.err)

			if tc.want.err == nil {
				test.AssertTerramateBlock(t, *got, tc.want.block)
			}
		})
	}
}
