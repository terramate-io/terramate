package hcl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/test"
)

func TestHCLParserModules(t *testing.T) {
	type want struct {
		modules   []hcl.Module
		err       error
		errPrefix error
	}
	type testcase struct {
		name  string
		input string
		want
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
		},
		{
			input: "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			want: want{
				errPrefix: fmt.Errorf("looking for \"test\".source attribute: " +
					"failed to evaluate"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := test.WriteFile(t, "", "main.tf", tc.input)

			parser := hcl.NewParser()
			modules, err := parser.ParseModules(path)
			if tc.want.errPrefix != nil {
				if err == nil {
					t.Fatalf("expects error prefix: %v", tc.want.errPrefix)
				}
				if !strings.HasPrefix(err.Error(), tc.want.errPrefix.Error()) {
					t.Fatalf("got[%v] but wants prefix [%v]", err, tc.want.errPrefix)
				}
			} else if tc.want.err != nil {
				assert.EqualErrs(t, tc.want.err, err, "failed to parse module %q", path)
			}

			assert.EqualInts(t, len(tc.modules), len(modules), "modules len mismatch")

			for i := 0; i < len(tc.want.modules); i++ {
				assert.EqualStrings(t, tc.want.modules[i].Source, modules[i].Source,
					"module source mismatch")
			}
		})
	}
}
