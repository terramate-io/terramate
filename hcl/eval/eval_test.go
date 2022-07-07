package eval_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/zclconf/go-cty/cty"
)

func TestEvalTmAbspath(t *testing.T) {
	type want struct {
		err   error
		value cty.Value
	}
	type testcase struct {
		name    string
		basedir string
		expr    string
		want    want
	}

	tempDir := t.TempDir()

	for _, tc := range []testcase{
		{
			name: "no args - fails",
			expr: `tm_abspath()`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "argument is a number - works ... mimicking terraform abspath()",
			expr: `tm_abspath(1)`,
			want: want{
				value: cty.StringVal("/1"),
			},
		},
		{
			name: "argument is slice - fails",
			expr: `tm_abspath([1])`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "absolute path is cleaned",
			expr: `tm_abspath("/test//something")`,
			want: want{
				value: cty.StringVal("/test/something"),
			},
		},
		{
			name: "relative path is appended to basedir",
			expr: `tm_abspath("something")`,
			want: want{
				value: cty.StringVal("/something"),
			},
		},
		{
			name: "relative path is cleaned",
			expr: `tm_abspath("something//")`,
			want: want{
				value: cty.StringVal("/something"),
			},
		},
		{
			name: "relative path with multiple levels is appended to basedir",
			expr: `tm_abspath("a/b/c/d/e")`,
			want: want{
				value: cty.StringVal("/a/b/c/d/e"),
			},
		},
		{
			name:    "empty path returns the basedir",
			expr:    `tm_abspath("")`,
			basedir: tempDir,
			want: want{
				value: cty.StringVal(tempDir),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			basedir := tc.basedir
			if basedir == "" {
				basedir = "/"
			}
			ctx, err := eval.NewContext(basedir)
			assert.NoError(t, err)

			const attrname = "value"

			cfg := fmt.Sprintf("%s = %s", attrname, tc.expr)
			parser := hclparse.NewParser()
			file, diags := parser.ParseHCL([]byte(cfg), "test-tm_abspath.hcl")
			if diags.HasErrors() {
				t.Fatalf("expr %q is not valid", tc.expr)
			}

			body := file.Body.(*hclsyntax.Body)
			attr := body.Attributes[attrname]

			got, err := ctx.Eval(attr.Expr)
			errtest.Assert(t, err, tc.want.err)
			if tc.want.err == nil {
				if !got.RawEquals(tc.want.value) {
					t.Fatalf("%#v != %#v", got, tc.want.value)
				}
			}
		})
	}
}
