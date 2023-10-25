// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval_test

import (
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/zclconf/go-cty/cty"
)

func TestPartialEval(t *testing.T) {
	t.Parallel()

	type testcase struct {
		expr    string
		want    string
		wantErr error
	}

	for _, tc := range []testcase{
		{
			expr: `1`,
		},
		{
			expr: `!1`,
		},
		{
			expr: `1.5`,
		},
		{
			expr: `1.2e10`,
			want: `12000000000`, // TODO(i4k): the scientific notation must be the expected output.
		},
		{
			expr: `"test"`,
		},
		{
			expr: `<<-EOT
test
EOT
`,
		},
		{
			expr: `"test ${unknown.val}"`,
		},
		{
			expr: `global.string`,
			want: `"terramate"`,
		},
		{
			expr: `!global.number`,
			want: `!10`,
		},
		{
			expr: `"test ${global.number}"`,
			want: `"test 10"`,
		},
		{
			expr: `"test ${global.string}"`,
			want: `"test terramate"`,
		},
		{
			expr: `[]`,
		},
		{
			expr: `[1, 2, 3]`,
		},
		{
			expr: `["terramate", "is", "fun"]`,
		},
		{
			expr: `[global.string, "is", "fun"]`,
			want: `["terramate", "is", "fun"]`,
		},
		{
			expr: `[!global.number]`,
			want: `[!10]`,
		},
		{
			expr: `{
				a = global.string
				b = "is"
				c = "fun"
			}`,
			want: `{
				a = "terramate"
				b = "is"
				c = "fun"
			}`,
		},
		{
			expr: `{
				a = {
					a = 1
					b = 2
					c = global.number
					d = global.string
					e = "test ${global.string} test2"
				}
				b = "is"
				c = "fun"
			}`,
			want: `{
				a = {
					a = 1
					b = 2
					c = 10
					d = "terramate"
					e = "test terramate test2"
				}
				b = "is"
				c = "fun"
			}`,
		},
		{
			expr: `true`,
		},
		{
			expr: `1+1`,
		},
		{
			expr: `1+data.val`,
		},
		{
			expr: `data.val1+1000+data.val2`,
		},
		{
			expr: `1+global.number`,
			want: `1+10`,
		},
		{
			expr: `global.string+global.number`,
			want: `"terramate"+10`,
		},
		{
			expr: `[1+global.number]`,
			want: `[1+10]`,
		},
		{
			expr: `{
				a = 1+global.number
			}`,
			want: `{
				a = 1+10
			}`,
		},
		{
			expr: `{
				global.string = 1
			}`,
			want: `{
				"terramate" = 1	
			}`,
		},
		{
			expr: `{
				global.obj.b[0] = 1
			}`,
			want: `{
				"terramate" = 1	
			}`,
		},
		{
			expr: `{
				global = 1
			}`,
			want: `{
				global = 1	
			}`,
		},
		{
			expr: `{
				(global) = 1
			}`,
			want: `{
				(global) = 1	
			}`,
		},
		{
			expr: `{
				(iter) = 1
			}`,
			want: `{
				(iter) = 1	
			}`,
		},
		{
			expr: `global`,
			want: `{
				list   = [0, 1, 2, 3]
				number = 10
				obj = {
				  a = 0
				  b = ["terramate"]
				}
				string  = "terramate"
				strings = ["terramate", "is", "fun"]
			}`,
		},
		{
			expr: `{
				(global.string) = 1
			}`,
			want: `{
				("terramate") = 1
			}`,
		},
		{
			expr: `{
				tm_upper(global.string) = 1
			}`,
			want: `{
				"TERRAMATE" = 1
			}`,
		},
		{
			expr: `{
				(tm_upper(global.string)) = 1
			}`,
			want: `{
				("TERRAMATE") = 1
			}`,
		},
		{
			expr: `{
				upper(global.string) = 1
			}`,
			want: `{
				upper("terramate") = 1
			}`,
		},
		{
			expr: `{
				upper("a") = 1
			}`,
			want: `{
				upper("a") = 1
			}`,
		},
		{
			expr: `{
				(upper("a")) = 1
			}`,
			want: `{
				(upper("a")) = 1
			}`,
		},
		{
			expr: `{
				a.b.c = 1
			}`,
			want: `{
				a.b.c = 1
			}`,
		},
		{
			expr: `funcall()`,
		},
		{
			expr: `funcall(1, 2, data.val, "test", {}, [1, 2])`,
		},
		{
			expr: `funcall(1, 2, global.number, 3)`,
			want: `funcall(1, 2, 10, 3)`,
		},
		{
			expr: `[for v in val : v if v+1 == data.something]`,
		},
		{
			expr: `{for k, v in val : k => funcall(v) if v+1 == data.something}`,
		},
		{
			expr:    `[for v in global.list : v if v+data == otherdata.val]`,
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			expr:    `[for v in global.strings : upper(v)]`,
			wantErr: errors.E(eval.ErrForExprDisallowEval),
		},
		{
			expr: `global.list[0]`,
			want: `0`,
		},
		{
			expr: `global.obj.a`,
			want: `0`,
		},
		{
			expr: `global.obj.b[0]`,
			want: `"terramate"`,
		},
		{
			expr: `true?[]:{}`,
		},
		{
			expr: `global.number == 10?global.string:global.list`,
			want: `10 == 10 ? "terramate" : [0, 1, 2, 3]`,
		},
		{
			expr: `[0, 1, 2][0]`,
			want: `0`,
		},
		{
			expr: `[0, 1, 2][1]`,
			want: `1`,
		},
		{
			expr: `{
				a = 1
			}.a`,
			want: `1`,
		},
		{
			expr: `tm_upper("terramate")`,
			want: `"TERRAMATE"`,
		},
		{
			expr: `tm_upper(global.string)`,
			want: `"TERRAMATE"`,
		},
	} {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			ctx := eval.NewContext(stdlib.Functions(os.TempDir()))
			ctx.SetNamespace("global", map[string]cty.Value{
				"number": cty.NumberIntVal(10),
				"string": cty.StringVal("terramate"),
				"list": cty.ListVal([]cty.Value{
					cty.NumberIntVal(0),
					cty.NumberIntVal(1),
					cty.NumberIntVal(2),
					cty.NumberIntVal(3),
				}),
				"strings": cty.ListVal([]cty.Value{
					cty.StringVal("terramate"),
					cty.StringVal("is"),
					cty.StringVal("fun"),
				}),
				"obj": cty.ObjectVal(map[string]cty.Value{
					"a": cty.NumberIntVal(0),
					"b": cty.ListVal([]cty.Value{cty.StringVal("terramate")}),
				}),
			})
			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatalf(diags.Error())
			}
			gotExpr, err := ctx.PartialEval(expr)
			errtest.Assert(t, err, tc.wantErr)
			if tc.wantErr != nil {
				return
			}
			want := tc.expr
			if tc.want != "" {
				want = tc.want
			}
			got := ast.TokensForExpression(gotExpr)
			assert.EqualStrings(t, string(hclwrite.Format([]byte(want))), string(hclwrite.Format(got.Bytes())))
		})
	}
}
