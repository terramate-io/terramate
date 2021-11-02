package hhcl_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
	"github.com/mineiros-io/terrastack/test"
)

func TestHHCLParserModules(t *testing.T) {
	type want struct {
		modules   []hcl.Module
		err       error
		errPrefix error
	}
	type testcase struct {
		input string
		want
	}

	var removeFiles []string

	defer func() {
		for _, f := range removeFiles {
			os.RemoveAll(f)
		}
	}()

	for _, tc := range []testcase{
		{
			input: `module {}`,
		},
		{
			input: `module "test" {}`,
		},
		{
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
			input: "a = 1\nmodule \"test\" {\nsource = \"test\"\n}\nb = 1",
			want: want{
				modules: []hcl.Module{
					{
						Source: "test",
					},
				},
			},
		},
		{
			input: "a = 1\nmodule \"test\" {\nsource = \"test\"\n}\nb = 1\n" +
				"module \"bleh\" {\nsource = \"bleh\"\n}\n",
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
			input: "module \"test\" {\nsource = -1\n}\n",
			want: want{
				err: fmt.Errorf("\"test\".source is not a string"),
			},
		},
		{
			input: "module \"test\" {\nsource = \"${var.test}\"\n}\n",
			want: want{
				errPrefix: fmt.Errorf("failed to evaluate \"test\".source attribute:"),
			},
		},
	} {
		path := test.CreateFile(t, "", "main.tf", tc.input)
		removeFiles = append(removeFiles, path)

		parser := hhcl.NewParser()
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
	}
}
