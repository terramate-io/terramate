// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
)

func TestStdlibTomlEncode(t *testing.T) {
	t.Parallel()
	type testcase struct {
		expr string
		want string
	}

	for _, tc := range []testcase{
		{
			expr: `tm_tomlencode(true)`,
			want: `true`,
		},
		{
			expr: `tm_tomlencode(1)`,
			want: `1.0`,
		},
		{
			expr: `tm_tomlencode(1.5)`,
			want: `1.5`,
		},
		{
			expr: `tm_tomlencode({})`,
			want: ``,
		},
		{
			expr: `tm_tomlencode({a = 1})`,
			want: nljoin(`a = 1.0`),
		},
		{
			expr: `tm_tomlencode({a = 1, b = "hello"})`,
			want: nljoin(
				`a = 1.0`,
				`b = 'hello'`,
			),
		},
	} {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{stdlib.TomlExperimentName}))
			val, err := ctx.Eval(test.NewExpr(t, tc.expr))
			assert.NoError(t, err)
			got := val.AsString()
			assert.EqualStrings(t, got, tc.want)
		})
	}
}

func TestStdlibTomlDecode(t *testing.T) {
	t.Parallel()
	type testcase struct {
		expr string
		want string
	}

	for _, tc := range []testcase{
		{
			expr: `tm_tomldecode(<<-EOF
			    project = "Terramate"
			  EOF
			)`,
			want: `{project = "Terramate"}`,
		},
		{
			expr: `tm_tomldecode(<<-EOF
			    age = 37
			  EOF
			)`,
			want: `{age = 37}`,
		},
		{
			expr: `tm_tomldecode(<<-EOF
			  [project]
			    name = "Terramate"
				url = "https://terramate.io"
			    created = 2021
				tags = ["iac", "terraform", "cloud"]
			  EOF
			)`,
			want: `{
			  project = {
			    name = "Terramate"
				url = "https://terramate.io"
				created = 2021
				tags = ["iac", "terraform", "cloud"]
			  }
		    }`,
		},
	} {
		tc := tc
		t.Run(tc.expr, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{stdlib.TomlExperimentName}))
			got, err := ctx.Eval(test.NewExpr(t, tc.expr))
			assert.NoError(t, err)
			gotStr := string(ast.TokensForValue(got).Bytes())
			wantExpr, err := ast.ParseExpression(tc.want, "want.hcl")
			assert.NoError(t, err)
			wantVal, err := ctx.Eval(wantExpr)
			assert.NoError(t, err)
			wantStr := string(ast.TokensForValue(wantVal).Bytes())
			assert.EqualStrings(t, gotStr, wantStr)
		})
	}
}
