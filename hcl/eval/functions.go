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

package eval

import (
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	tflang "github.com/hashicorp/terraform/lang"
	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func newTmFunctions(basedir string) map[string]function.Function {
	scope := &tflang.Scope{BaseDir: basedir}
	tffuncs := scope.Functions()

	tmfuncs := map[string]function.Function{}
	for name, function := range tffuncs {
		tmfuncs["tm_"+name] = function
	}

	// fix terraform broken abspath()
	tmfuncs["tm_abspath"] = tmAbspath(basedir)

	// sane ternary
	tmfuncs["tm_ternary"] = tmTernary()
	return tmfuncs
}

// tmAbspath returns the `tm_abspath()` hcl function.
func tmAbspath(basedir string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			path := args[0].AsString()
			var abspath string
			if filepath.IsAbs(path) {
				abspath = path
			} else {
				abspath = filepath.Join(basedir, path)
			}

			return cty.StringVal(filepath.ToSlash(filepath.Clean(abspath))), nil
		},
	})
}

func tmTernary() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "cond",
				Type: cty.Bool,
			},
			{
				Name: "val1",
				Type: customdecode.ExpressionClosureType,
			},
			{
				Name: "val2",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: func(args []cty.Value) (cty.Type, error) {
			v, err := ternary(args[0], args[1], args[2])
			if err != nil {
				return cty.NilType, err
			}
			return v.Type(), nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return ternary(args[0], args[1], args[2])
		},
	})
}

func ternary(cond cty.Value, val1, val2 cty.Value) (cty.Value, error) {
	if cond.True() {
		return evalTernaryBranch(val1)
	}
	return evalTernaryBranch(val2)
}

func evalTernaryBranch(arg cty.Value) (cty.Value, error) {
	closure := customdecode.ExpressionClosureFromVal(arg)

	ctx := NewContextFrom(closure.EvalContext)
	newtokens, err := ctx.PartialEval(closure.Expression)
	if err != nil {
		return cty.NilVal, errors.E(err, "evaluating tm_ternary branch")
	}

	exprParsed, err := parseExpressionBytes(newtokens.Bytes())
	if err != nil {
		return cty.NilVal, errors.E(err, "parsing partial evaluated bytes")
	}

	if dependsOnUnknowns(exprParsed, closure.EvalContext) {
		return customdecode.ExpressionVal(exprParsed), nil
	}

	v, diags := exprParsed.Value(closure.EvalContext)
	if diags.HasErrors() {
		return cty.NilVal, errors.E(diags, "evaluating tm_ternary branch")
	}

	return v, nil
}

// dependsOnUnknowns returns true if any of the variables that the given
// expression might access are unknown values or contain unknown values.
func dependsOnUnknowns(expr hcl.Expression, ctx *hcl.EvalContext) bool {
	for _, traversal := range expr.Variables() {
		val, diags := traversal.TraverseAbs(ctx)
		if diags.HasErrors() {
			return true
		}
		if !val.IsWhollyKnown() {
			return true
		}
	}
	return false
}
