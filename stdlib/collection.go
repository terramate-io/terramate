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

// AllTrueFunc implements the `tm_alltrue()` function.
func AllTrueFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
			argClosure := customdecode.ExpressionClosureFromVal(args[0])
			evalctx := eval.NewContextFrom(argClosure.EvalContext)
			listExpression, ok := argClosure.Expression.(*hclsyntax.TupleConsExpr)
			if !ok {
				v, err := evalctx.Eval(argClosure.Expression)
				if err != nil {
					return cty.False, err
				}
				if !v.Type().IsListType() && !v.Type().IsTupleType() {
					return cty.False, errors.E(`Invalid value for "list" parameter: %s`, v.Type().FriendlyName())
				}
				result := true
				i := 0
				for it := v.ElementIterator(); it.Next(); {
					_, v := it.Element()
					if !v.IsKnown() {
						return cty.UnknownVal(cty.Bool), nil
					}
					if v.IsNull() {
						return cty.False, nil
					}
					if !v.Type().Equals(cty.Bool) {
						return cty.False, errors.E(`Invalid value for "list" parameter: element %d: bool required`, i+1)
					}
					result = result && v.True()
					if !result {
						return cty.False, nil
					}
					i++
				}
				return cty.True, nil
			}

			result := true
			for i, expr := range listExpression.Exprs {
				v, err := evalctx.Eval(expr)
				if err != nil {
					return cty.False, err
				}
				if v.IsNull() {
					return cty.False, nil
				}
				if !v.Type().Equals(cty.Bool) {
					return cty.False, errors.E(`Invalid value for "list" parameter: element %d: bool required`, i+1)
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

// AnyTrueFunc implements the `tm_anytrue()` function.
func AnyTrueFunc() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "list",
				Type: customdecode.ExpressionClosureType,
			},
		},
		Type: function.StaticReturnType(cty.Bool),
		Impl: func(args []cty.Value, retType cty.Type) (ret cty.Value, err error) {
			argClosure := customdecode.ExpressionClosureFromVal(args[0])
			evalctx := eval.NewContextFrom(argClosure.EvalContext)
			listExpression, ok := argClosure.Expression.(*hclsyntax.TupleConsExpr)
			if !ok {
				v, err := evalctx.Eval(argClosure.Expression)
				if err != nil {
					return cty.False, err
				}
				if !v.Type().IsListType() && !v.Type().IsTupleType() {
					return cty.False, errors.E(`Invalid value for "list" parameter: %s`, v.Type().FriendlyName())
				}
				result := false
				i := 0
				for it := v.ElementIterator(); it.Next(); {
					_, v := it.Element()
					if !v.IsKnown() {
						continue
					}
					if v.IsNull() {
						continue
					}
					if !v.Type().Equals(cty.Bool) {
						return cty.False, errors.E(`Invalid value for "list" parameter: element %d: bool required`, i+1)
					}
					result = result || v.True()
					if result {
						return cty.True, nil
					}
					i++
				}
				return cty.False, nil
			}

			result := false
			for i, expr := range listExpression.Exprs {
				v, err := evalctx.Eval(expr)
				if err != nil {
					return cty.False, err
				}
				if v.IsNull() {
					continue
				}
				if !v.Type().Equals(cty.Bool) {
					return cty.False, errors.E(`Invalid value for "list" parameter: element %d: bool required`, i+1)
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
