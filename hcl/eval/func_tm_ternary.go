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
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

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
	evalExprVal := func(arg cty.Value) (cty.Value, error) {
		closure := customdecode.ExpressionClosureFromVal(arg)
		if dependsOnUnknowns(closure.Expression, closure.EvalContext) {
			return arg, nil
		}

		v, diags := closure.Value()
		if diags.HasErrors() {
			return cty.NilVal, diags
		}
		return v, nil
	}

	if cond.True() {
		return evalExprVal(val1)
	}
	return evalExprVal(val2)
}

// dependsOnUnknowns returns true if any of the variables that the given
// expression might access are unknown values or contain unknown values.
//
// This is a conservative result that prefers to return true if there's any
// chance that the expression might derive from an unknown value during its
// evaluation; it is likely to produce false-positives for more complex
// expressions involving deep data structures.
func dependsOnUnknowns(expr hcl.Expression, ctx *hcl.EvalContext) bool {
	for _, traversal := range expr.Variables() {
		val, diags := traversal.TraverseAbs(ctx)
		if diags.HasErrors() {
			return true
		}
		if !val.IsWhollyKnown() {
			// The value will be unknown if either it refers directly to
			// an unknown value or if the traversal moves through an unknown
			// collection. We're using IsWhollyKnown, so this also catches
			// situations where the traversal refers to a compound data
			// structure that contains any unknown values. That's important,
			// because during evaluation the expression might evaluate more
			// deeply into this structure and encounter the unknowns.
			return true
		}
	}
	return false
}
