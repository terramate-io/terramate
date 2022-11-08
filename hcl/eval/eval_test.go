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
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
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
		t.Run(tc.name, func(t *testing.T) {
			basedir := tc.basedir
			if basedir == "" {
				basedir = root(t)
			}
			ctx, err := eval.NewContext(basedir, project.NewPath("/"))
			assert.NoError(t, err)

			const attrname = "value"

			cfg := fmt.Sprintf("%s = %s", attrname, strings.ReplaceAll(tc.expr, `\`, `\\`))
			fname := test.WriteFile(t, t.TempDir(), "test-tm_ternary.hcl", cfg)
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

func TestEvalTmAbspathMustPanicIfRelativeBaseDir(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("eval.NewContext() did not panic with relative basedir")
		}
	}()
	_, _ = eval.NewContext("relative", project.NewPath("/"))
}

func TestEvalTmAbspathFailIfBasedirIsNonExistent(t *testing.T) {
	_, err := eval.NewContext(t.TempDir(), project.NewPath("/non-existent"))
	assert.Error(t, err, "must have failed for non-existent basedir")
}

func TestEvalTmAbspathFailIfBasedirIsNotADirectory(t *testing.T) {
	path := test.WriteFile(t, t.TempDir(), "somefile.txt", ``)
	_, err := eval.NewContext(filepath.Dir(path), project.NewPath("/somefile.txt"))
	assert.Error(t, err, "must have failed because basedir is not a directory")
}
