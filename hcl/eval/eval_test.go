// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/zclconf/go-cty/cty"
)

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

func TestEvalTmFuncall(t *testing.T) {
	t.Parallel()
	tcases := []testcase{
		{
			name: "tm_ternary - cond is true, with primitive values",
			expr: `tm_ternary(true, "hello", "world")`,
			want: want{
				value: cty.StringVal("hello"),
			},
		},
		{
			name: "tm_ternary - cond is false, with primitive values",
			expr: `tm_ternary(false, "hello", "world")`,
			want: want{
				value: cty.StringVal("world"),
			},
		},
		{
			name: "tm_ternary - cond is false, with partial not evaluated",
			expr: `tm_ternary(false, local.var, "world")`,
			want: want{
				value: cty.StringVal("world"),
			},
		},
		{
			name: "no args - fails",
			expr: `tm_abspath()`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "argument is slice - fails",
			expr: `tm_abspath([1])`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
	}

	tcases = append(tcases, tmAbspathTestcases(t)...)

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			basedir := tc.basedir
			if basedir == "" {
				basedir = root(t)
			}
			ctx := eval.NewContext(stdlib.Functions(basedir))

			const attrname = "value"

			cfg := fmt.Sprintf("%s = %s", attrname, strings.ReplaceAll(tc.expr, `\`, `\\`))
			fname := test.WriteFile(t, test.TempDir(t), "test-tm_ternary.hcl", cfg)
			parser := hclparse.NewParser()
			file, diags := parser.ParseHCL([]byte(cfg), fname)
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
