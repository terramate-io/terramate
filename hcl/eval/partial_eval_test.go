// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/hcl/v2/hclwrite"
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
		expr        string
		want        string
		hasUnknowns bool
		wantErr     error
	}

	for _, tc := range []testcase{
		{
			expr: `1`,
		},
		{
			expr: `!1`,
			//  Unsuitable value for unary operand: bool required.
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `true`,
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
			expr:        `"test ${unknown.val}"`,
			hasUnknowns: true,
		},
		{
			expr: `global.string`,
			want: `"terramate"`,
		},
		{
			expr:    `!global.number`,
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `!global.falsy`,
			want: `true`,
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
			// Unsuitable value for unary operand: bool required.
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `[!global.falsy]`,
			want: `[true]`,
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
			want: `2`,
		},
		{
			expr:        `1+data.val`,
			hasUnknowns: true,
		},
		{
			expr:        `data.val1+1000+data.val2`,
			hasUnknowns: true,
		},
		{
			expr: `1+global.number`,
			want: `11`,
		},
		{
			expr: `global.string+global.number`,
			// Unsuitable value for left operand: a number is required.
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `global.number+global.number`,
			want: `20`,
		},
		{
			expr: `[1+global.number]`,
			want: `[11]`,
		},
		{
			expr: `{
				a = 1+global.number
			}`,
			want: `{
				a = 11
			}`,
		},
		{
			expr: `{
				global.string = 1
			}`,
			want: `{
				terramate = 1	
			}`,
		},
		{
			expr: `{
				global.obj.b[0] = 1
			}`,
			want: `{
				terramate = 1	
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
			// Can't use this value as a key: string required
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `{
				(aws.vpc.id) = 1
			}`,
			want: `{
				(aws.vpc.id) = 1	
			}`,
			hasUnknowns: true,
		},
		{
			expr: `{
				(iter) = 1
			}`,
			want: `{
				(iter) = 1	
			}`,
			hasUnknowns: true,
		},
		{
			expr: `global`,
			want: `{
				falsy  = false
				list   = [0, 1, 2, 3]
				number = 10
				obj = {
				  a = 0
				  b = ["terramate"]
				}
				string  = "terramate"
				strings = ["terramate", "is", "fun"]
				truer   = true
			  }`,
		},
		{
			expr: `{
				(global.string) = 1
			}`,
			want: `{
				terramate = 1
			}`,
		},
		{
			expr: `{
				tm_upper(global.string) = 1
			}`,
			want: `{
				TERRAMATE = 1
			}`,
		},
		{
			expr: `{
				(tm_upper(global.string)) = 1
			}`,
			want: `{
				TERRAMATE = 1
			}`,
		},
		{
			expr: `{
				upper(global.string) = 1
			}`,
			want: `{
				upper("terramate") = 1
			}`,
			hasUnknowns: true,
		},
		{
			expr: `{
				upper("a") = 1
			}`,
			want: `{
				upper("a") = 1
			}`,
			hasUnknowns: true,
		},
		{
			expr: `{
				(upper("a")) = 1
			}`,
			want: `{
				(upper("a")) = 1
			}`,
			hasUnknowns: true,
		},
		{
			expr: `{
				a.b.c = 1
			}`,
			want: `{
				a.b.c = 1
			}`,
			hasUnknowns: true,
		},
		{
			expr:        `funcall()`,
			hasUnknowns: true,
		},
		{
			expr:        `funcall(1, 2, data.val, "test", {}, [1, 2])`,
			hasUnknowns: true,
		},
		{
			expr:        `funcall(1, 2, global.number, 3)`,
			want:        `funcall(1, 2, 10, 3)`,
			hasUnknowns: true,
		},
		{
			expr:        `[for v in val : v if v+1 == data.something]`,
			hasUnknowns: true,
		},
		{
			expr:        `{for k, v in val : k => funcall(v) if v+1 == data.something}`,
			hasUnknowns: true,
		},
		{
			expr:        `[for v in global.list : v if v+data == otherdata.val]`,
			want:        `[for v in [0, 1, 2, 3] : v if v + data == otherdata.val]`,
			hasUnknowns: true,
		},
		{
			expr: `[for v in global.strings : tm_upper(v)]`,
			want: `["TERRAMATE", "IS", "FUN"]`,
		},
		{
			expr: `{for k, v in global.obj : tm_upper(k) => v if k == "a"}`,
			want: `{
				A = 0
			}`,
		},
		{
			expr: `{for k, v in global.obj : k => v+100 if k == "a"}`,
			want: `{
				a = 100
			}`,
		},
		{
			// loop cannot be evaluated because `coll` refers to unknowns
			expr: `{for k, v in unknown_func(global.obj) : k => v if var.data}`,
			want: `{for k, v in unknown_func({
				a = 0
				b = ["terramate"]
			  }) : k => v if var.data}`,
			hasUnknowns: true,
		},
		{
			// loop cannot be evaluated because `if ...` refers to unknowns
			expr: `{for k, v in global.obj : k => v if var.data}`,
			want: `{for k, v in {
				a = 0
				b = ["terramate"]
			  } : k => v if var.data}`,
			hasUnknowns: true,
		},
		{
			// loop cannot be evaluated because `if ...` refers to unknowns function
			expr: `{for k, v in global.obj : k => v if unknown_func(v)}`,
			want: `{for k, v in {
				a = 0
				b = ["terramate"]
			  } : k => v if unknown_func(v)}`,
			hasUnknowns: true,
		},
		{
			// loop cannot be reduced because `key` refers to unknowns function
			expr: `{for k, v in global.obj : unknown_func(k) => v}`,
			want: `{for k, v in {
				a = 0
				b = ["terramate"]
			  } : unknown_func(k) => v}`,
			hasUnknowns: true,
		},
		{
			// loop cannot be reduced because `value` refers to unknowns function
			expr: `{for k, v in global.obj : k => unknown_func(v)}`,
			want: `{for k, v in {
				a = 0
				b = ["terramate"]
			  } : k => unknown_func(v)}`,
			hasUnknowns: true,
		},
		{
			expr: `{for k, v in global.obj : unknown => v if k == "a"}`,
			want: `{ for k, v in {
				a = 0
				b = ["terramate"]
			  } : unknown => v if k == "a" }`,
			hasUnknowns: true,
		},
		{
			expr:    `{for k, v in global.obj : v => v+1 if k == "a"}`,
			wantErr: errors.E(eval.ErrPartial), // v is a number
		},
		{
			expr:    `{for k, v in global.obj : v => v+1 if k == "a"}`,
			wantErr: errors.E(eval.ErrPartial), // v is a number
		},
		{
			expr:        `[for v in tm_concat(global.list, [4, 5, 6]) : v if v+data == otherdata.val]`,
			want:        `[for v in [0, 1, 2, 3, 4, 5, 6] : v if v + data == otherdata.val]`,
			hasUnknowns: true,
		},
		{
			expr: `{for k in data.something : k => global.obj[k]}`,
			want: `{for k in data.something : k => {
				a = 0
				b = ["terramate"]
			  }[k]}`,
			hasUnknowns: true,
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
			// different branch types
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `true?"truth":"fake"`,
			want: `"truth"`,
		},
		{
			expr: `false?"truth":"fake"`,
			want: `"fake"`,
		},
		{
			expr: `global.number == 10?global.string:global.list`,
			// err: eval expression: The true and false result expressions must have consistent types. The 'true' value is string, but the 'false' value is list of number.
			wantErr: errors.E(eval.ErrPartial),
		},
		{
			expr: `global.number == 10?global.string:"test"`,
			want: `"terramate"`,
		},
		{
			expr: `[0, 1, 2][0]`,
			want: `0`,
		},
		// fuzzer case
		{
			expr:        `A.0[0.*]`,
			want:        `A[0][[0]]`,
			hasUnknowns: true,
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
			ctx := eval.NewContext(stdlib.Functions(os.TempDir(), []string{}))
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
				"truer": cty.True,
				"falsy": cty.False,
			})
			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatalf(diags.Error())
			}
			gotExpr, hasUnknowns, err := ctx.PartialEval(expr)
			errtest.Assert(t, err, tc.wantErr)
			if tc.wantErr != nil {
				return
			}
			want := tc.expr
			if tc.want != "" {
				want = tc.want
			}
			got := ast.TokensForExpression(gotExpr)
			wantFormatted := string(hclwrite.Format([]byte(want)))
			gotFormatted := string(hclwrite.Format(got.Bytes()))
			t.Logf("got:  '%s'", gotFormatted)
			t.Logf("want: '%s'", wantFormatted)
			if diff := cmp.Diff(wantFormatted, gotFormatted); diff != "" {
				t.Fatal(diff)
			}
			if hasUnknowns != tc.hasUnknowns {
				t.Fatalf("hasUnknowns mismatch: got[%t] != want[%t]", hasUnknowns, tc.hasUnknowns)
			}
		})
	}
}
