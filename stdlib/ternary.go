// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/customdecode"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// TernaryFunc is the `tm_ternary` function implementation.
// The `tm_ternary(cond, expr1, expr2)` will return expr1 if `cond` evaluates
// to `true` and `expr2` otherwise.
func TernaryFunc(evalctx *eval.Context) function.Function {
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
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return ternary(evalctx, args[0], args[1], args[2])
		},
	})
}

func AllTrueFunc(evalctx *eval.Context) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
			if len(args) > 1 {
				panic("todo: only 1 argument")
			}

			argClosure := customdecode.ExpressionClosureFromVal(args[0])
			listExpression, ok := argClosure.Expression.(*hclsyntax.TupleConsExpr)
			if !ok {
				v, err := evalctx.Eval(argClosure.Expression)
				if err != nil {
					return cty.False, err
				}
				if !v.Type().IsTupleType() {
					return cty.False, errors.E("todo: not a list: %s", v.Type().FriendlyName())
				}
				result := true
				for it := v.ElementIterator(); it.Next(); {
					_, v := it.Element()
					if !v.IsKnown() {
						return cty.UnknownVal(cty.Bool), nil
					}
					if v.IsNull() {
						return cty.False, nil
					}
					if !v.Type().Equals(cty.Bool) {
						return cty.False, errors.E("not a boolean")
					}
					result = result && v.True()
					if !result {
						return cty.False, nil
					}
				}
				return cty.True, nil
			}

			result := true
			for _, expr := range listExpression.Exprs {
				v, err := evalctx.Eval(expr)
				if err != nil {
					return cty.False, errors.E(err, "evaluating tm_alltrue() arguments")
				}
				if v.IsNull() {
					return cty.False, nil
				}
				if !v.Type().Equals(cty.Bool) {
					return cty.False, errors.E("tm_alltrue() argument element is not a bool")
				}
				result = result && v.True()
				if !result {
					return cty.False, nil
				}
			}
			return cty.True, nil
		},
	})
}

func AnyTrueFunc(evalctx *eval.Context) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
			if len(args) > 1 {
				panic("todo: only 1 argument")
			}

			argClosure := customdecode.ExpressionClosureFromVal(args[0])
			listExpression, ok := argClosure.Expression.(*hclsyntax.TupleConsExpr)
			if !ok {
				v, err := evalctx.Eval(argClosure.Expression)
				if err != nil {
					return cty.False, err
				}
				if !v.Type().IsTupleType() {
					return cty.False, errors.E("todo: not a list: %s", v.Type().FriendlyName())
				}
				result := false
				for it := v.ElementIterator(); it.Next(); {
					_, v := it.Element()
					if !v.IsKnown() {
						continue
					}
					if v.IsNull() {
						continue
					}
					if !v.Type().Equals(cty.Bool) {
						return cty.False, errors.E("not a boolean")
					}
					result = result || v.True()
					if result {
						return cty.True, nil
					}
				}
				return cty.False, nil
			}

			result := false
			for _, expr := range listExpression.Exprs {
				v, err := evalctx.Eval(expr)
				if err != nil {
					return cty.False, errors.E(err, "evaluating tm_alltrue() arguments")
				}
				if v.IsNull() {
					continue
				}
				if !v.Type().Equals(cty.Bool) {
					return cty.False, errors.E("tm_alltrue() argument element is not a bool")
				}
				result = result || v.True()
				if result {
					return cty.True, nil
				}
			}
			return cty.False, nil
		},
	})
}

func ternary(evalctx *eval.Context, cond cty.Value, val1, val2 cty.Value) (cty.Value, error) {
	if cond.True() {
		return evalTernaryBranch(evalctx, val1)
	}
	return evalTernaryBranch(evalctx, val2)
}

func evalTernaryBranch(evalctx *eval.Context, arg cty.Value) (cty.Value, error) {
	closure := customdecode.ExpressionClosureFromVal(arg)
	bk := evalctx.Hclctx
	evalctx.Hclctx = closure.EvalContext
	newexpr, err := evalctx.PartialEval(&ast.CloneExpression{
		Expression: closure.Expression.(hclsyntax.Expression),
	})
	evalctx.Hclctx = bk
	if err != nil {
		return cty.NilVal, errors.E(err, "evaluating tm_ternary branch")
	}

	if dependsOnUnknowns(newexpr, closure.EvalContext) {
		return customdecode.ExpressionVal(newexpr), nil
	}

	v, diags := newexpr.Value(closure.EvalContext)
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
