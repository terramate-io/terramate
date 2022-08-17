// Copyright 2022 Mineiros GmbH
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
	"fmt"
	"path/filepath"
	"testing"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/test"
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

	localVarExpr, _ := hclsyntax.ParseExpression([]byte(`local.var`), "gen.hcl", hhcl.Pos{})

	for _, tc := range []testcase{
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
			name: "tm_ternary - cond is true, with partial returning",
			expr: `tm_ternary(true, local.var, "world")`,
			want: want{
				value: customdecode.ExpressionClosureVal(&customdecode.ExpressionClosure{
					Expression: localVarExpr,
				}),
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

			// hack to provide the same context to both closures.
			if tc.want.value.Type() == customdecode.ExpressionClosureType {
				underData := tc.want.value.EncapsulatedValue()
				if underData != nil {
					closure := underData.(*customdecode.ExpressionClosure)
					closure.EvalContext = ctx.Hclctx
				}
			}
			errtest.Assert(t, err, tc.want.err)
			if tc.want.err == nil {
				if !got.RawEquals(tc.want.value) {
					t.Fatalf("%#v != %#v", got, tc.want.value)
				}
			}
		})
	}
}

func TestEvalTmAbspathMustPanicIfRelativeBaseDir(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("eval.NewContext() did not panic with relative basedir")
		}
	}()
	_, _ = eval.NewContext("relative")
}

func TestEvalTmAbspathFailIfBasedirIsNonExistent(t *testing.T) {
	_, err := eval.NewContext(filepath.Join(t.TempDir(), "non-existent"))
	assert.Error(t, err, "must have failed for non-existent basedir")
}

func TestEvalTmAbspathFailIfBasedirIsNotADirectory(t *testing.T) {
	path := test.WriteFile(t, t.TempDir(), "somefile.txt", ``)
	_, err := eval.NewContext(path)
	assert.Error(t, err, "must have failed because basedir is not a directory")
}
