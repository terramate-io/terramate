// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestStdlibHCLEncode(t *testing.T) {
	t.Parallel()
	type want struct {
		res string
		err error
	}
	type testcase struct {
		name string
		expr string
		want want
	}

	for _, tc := range []testcase{
		{
			name: "only map/object can be encoded",
			expr: `tm_hclencode(bool)`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "empty object/map generates an empty string",
			expr: `tm_hclencode({})`,
			want: want{
				res: ``,
			},
		},
		{
			name: "only object with invalid keys",
			expr: `tm_hclencode({"this is not valid" = 1})`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "only object with invalid number key",
			expr: `tm_hclencode({123 = "value"})`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "encoding numbers",
			expr: `tm_hclencode({a = 1})`,
			want: want{
				res: nljoin(`a = 1`),
			},
		},
		{
			name: "encoding string",
			expr: `tm_hclencode({abc = "string"})`,
			want: want{
				res: nljoin(`abc = "string"`),
			},
		},
		{
			name: "encoding object with an empty object",
			expr: `tm_hclencode({obj = {}})`,
			want: want{
				res: nljoin(`obj = {}`),
			},
		},
		{
			name: "encoding object with an tuples",
			expr: `tm_hclencode({
				obj = {list = [1, 2]}
			})`,
			want: want{
				res: nljoin(`obj = {
  list = [
    1,
    2,
  ]
}`),
			},
		},
		{
			name: "encoding the result of the decoder works as expected",
			expr: `tm_hclencode(tm_hcldecode(tm_hclencode({number = 10, string = "string"})))`,
			want: want{
				res: nljoin(`number = 10`, `string = "string"`),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
			val, err := ctx.Eval(test.NewExpr(t, tc.expr))
			errtest.Assert(t, err, tc.want.err)
			if tc.want.err != nil {
				return
			}
			got := val.AsString()
			if diff := cmp.Diff(tc.want.res, got); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestStdlibHCLDecode(t *testing.T) {
	t.Parallel()
	type want struct {
		err error
		res string
	}
	type testcase struct {
		name string
		expr string
		want want
	}

	for _, tc := range []testcase{
		{
			name: "functions are not supported",
			expr: `tm_hcldecode(<<-EOF
			name = tm_upper("Terramate")
		  EOF
		)`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "blocks are not supported",
			expr: `tm_hcldecode(<<-EOF
			someBlock {
			   a = 1
			}
		  EOF
		)`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "string assignment",
			expr: `tm_hcldecode(<<-EOF
			    project = "Terramate"
			  EOF
			)`,
			want: want{
				res: `{project = "Terramate"}`,
			},
		},
		{
			name: "number assignment",
			expr: `tm_hcldecode(<<-EOF
			    age = 37
			  EOF
			)`,
			want: want{
				res: `{age = 37}`,
			},
		},
		{
			name: "object assignment",
			expr: `tm_hcldecode(<<-EOF
			  project = {
			    name = "Terramate"
				url = "https://terramate.io"
			    created = 2021
				tags = ["iac", "terraform", "cloud"]
		      }
			  EOF
			)`,
			want: want{
				res: `{
			  project = {
			    name = "Terramate"
				url = "https://terramate.io"
				created = 2021
				tags = ["iac", "terraform", "cloud"]
			  }
		    }`,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
			got, err := ctx.Eval(test.NewExpr(t, tc.expr))
			errtest.Assert(t, err, tc.want.err)
			if tc.want.err != nil {
				return
			}
			assert.NoError(t, err)
			gotStr := string(ast.TokensForValue(got).Bytes())
			wantExpr, err := ast.ParseExpression(tc.want.res, "want.hcl")
			assert.NoError(t, err)
			wantVal, err := ctx.Eval(wantExpr)
			assert.NoError(t, err)
			wantStr := string(ast.TokensForValue(wantVal).Bytes())
			assert.EqualStrings(t, gotStr, wantStr)
		})
	}
}
