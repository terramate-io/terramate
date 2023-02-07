// Copyright 2023 Mineiros GmbH
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

package eval_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

func TestPartialEval2(t *testing.T) {
	type testcase struct {
		expr string
		want string
	}

	for _, tc := range []testcase{
		{
			expr: `1`,
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
			expr: `"test ${global.number}"`,
			want: `"test ${10}"`,
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
	} {
		ctx := eval.NewContext(nil)
		ctx.SetNamespace("global", map[string]cty.Value{
			"number": cty.NumberIntVal(10),
			"string": cty.StringVal("terramate"),
		})
		expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			t.Fatalf(diags.Error())
		}
		eval.Experimental = true
		got, err := ctx.PartialEval(expr)
		assert.NoError(t, err)
		want := tc.expr
		if tc.want != "" {
			want = tc.want
		}
		assert.EqualStrings(t, want, string(hclwrite.Format(got.Bytes())))
	}
}

func BenchmarkPartialEval(b *testing.B) {
	b.StopTimer()
	ctx := eval.NewContext(nil)
	ctx.SetNamespace("global", map[string]cty.Value{
		"number": cty.NumberIntVal(11),
		"string": cty.StringVal("terramate"),
	})
	exprStr := `"${global.string} v0.2.${global.number} is a Terraform Orchestration and Code Generation tool"`
	exprV1, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		b.Fatalf(diags.Error())
	}

	exprV2, diags := hclsyntax.ParseExpression([]byte(exprStr), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		b.Fatalf(diags.Error())
	}
	b.Run("v1", func(b *testing.B) {
		eval.Experimental = false
		b.StartTimer()
		for n := 0; n < b.N; n++ {
			_, err := ctx.PartialEval(exprV1)
			if err != nil {
				panic(err)
			}
		}
	})

	b.Run("v2", func(b *testing.B) {
		eval.Experimental = true
		b.StartTimer()
		for n := 0; n < b.N; n++ {
			_, err := ctx.PartialEval(exprV2)
			if err != nil {
				panic(err)
			}
		}
	})

}
