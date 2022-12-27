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

package stdlib

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// TernaryFunc is the `tm_ternary` function implementation.
// The `tm_ternary(cond, expr1, expr2)` will return expr1 if `cond` evaluates
// to `true` and `expr2` otherwise.
func TernaryFunc() function.Function {
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

	ctx := eval.NewContextFrom(closure.EvalContext)
	newtokens, err := ctx.PartialEval(closure.Expression)
	if err != nil {
		return cty.NilVal, errors.E(err, "evaluating tm_ternary branch")
	}

	exprParsed, err := eval.ParseExpressionBytes(newtokens.Bytes())
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
