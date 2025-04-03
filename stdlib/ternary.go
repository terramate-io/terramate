// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"github.com/terramate-io/hcl/v2/ext/customdecode"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
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
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
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
	newexpr, hasUnknowns, err := ctx.PartialEval(closure.Expression.(hclsyntax.Expression))
	if err != nil {
		return cty.NilVal, errors.E(err, "evaluating tm_ternary branch")
	}

	if hasUnknowns {
		return customdecode.ExpressionVal(newexpr), nil
	}

	v, diags := newexpr.Value(closure.EvalContext)
	if diags.HasErrors() {
		return cty.NilVal, errors.E(diags, "evaluating tm_ternary branch")
	}

	return v, nil
}
