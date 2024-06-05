// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"fmt"
	"strings"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned when doing partial evaluation.
const (
	ErrPartial       errors.Kind = "partial evaluation failed"
	ErrInterpolation errors.Kind = "interpolation failed"
)

func (c *Context) partialEval(expr hhcl.Expression) (newexpr hhcl.Expression, hasUnknowns bool, err error) {
	switch e := expr.(type) {
	case *ast.CloneExpression:
		cloned := ast.CloneExpr(e.Expression)
		return c.partialEval(cloned)
	case *hclsyntax.LiteralValueExpr:
		return expr, false, nil
	case *hclsyntax.UnaryOpExpr:
		return c.partialEvalUnaryOp(e)
	case *hclsyntax.BinaryOpExpr:
		return c.partialEvalBinOp(e)
	case *hclsyntax.ConditionalExpr:
		return c.partialEvalCondExpr(e)
	case *hclsyntax.TupleConsExpr:
		return c.partialEvalTuple(e)
	case *hclsyntax.ObjectConsExpr:
		return c.partialEvalObject(e)
	case *hclsyntax.FunctionCallExpr:
		return c.partialEvalFunc(e)
	case *hclsyntax.IndexExpr:
		return c.partialEvalIndex(e)
	case *hclsyntax.SplatExpr:
		return c.partialEvalSplat(e)
	case *hclsyntax.ForExpr:
		return c.partialEvalForExpr(e)
	case *hclsyntax.ObjectConsKeyExpr:
		return c.partialEvalObjectKey(e)
	case *hclsyntax.TemplateExpr:
		return c.partialEvalTemplate(e)
	case *hclsyntax.TemplateWrapExpr:
		return c.partialEvalTmplWrap(e)
	case *hclsyntax.ScopeTraversalExpr:
		return c.partialEvalScopeTrav(e)
	case *hclsyntax.RelativeTraversalExpr:
		return c.partialEvalRelTrav(e)
	case *hclsyntax.ParenthesesExpr:
		return c.partialEvalParenExpr(e)
	case *hclsyntax.AnonSymbolExpr:
		return expr, false, nil
	default:
		panic(fmt.Sprintf("not implemented %T", expr))
	}
}

func (c *Context) partialEvalTemplate(tmpl *hclsyntax.TemplateExpr) (*hclsyntax.TemplateExpr, bool, error) {
	hasUnknowns := false
	for i, part := range tmpl.Parts {
		newexpr, partHasUnknowns, err := c.partialEval(part)
		if err != nil {
			return nil, false, err
		}
		hasUnknowns = hasUnknowns || partHasUnknowns
		tmpl.Parts[i] = asSyntax(newexpr)
	}
	return tmpl, hasUnknowns, nil
}

func (c *Context) partialEvalTmplWrap(wrap *hclsyntax.TemplateWrapExpr) (hhcl.Expression, bool, error) {
	newwrap, hasUnknowns, err := c.partialEval(wrap.Wrapped)
	if err != nil {
		return nil, false, err
	}
	if v, ok := newwrap.(*hclsyntax.LiteralValueExpr); ok {
		// TODO(i4k): check this case.
		if v.Val.Type() == cty.String && strings.Contains(v.Val.AsString(), "${") {
			panic(errors.E(errors.ErrInternal, "unexpected case: %s", v.Val.AsString()))
		}
		return v, hasUnknowns, nil
	}

	wrap.Wrapped = asSyntax(newwrap)
	return wrap, hasUnknowns, nil
}

func (c *Context) partialEvalTuple(tuple *hclsyntax.TupleConsExpr) (hclsyntax.Expression, bool, error) {
	hasUnknowns := false
	for i, v := range tuple.Exprs {
		newexpr, itHasUnknowns, err := c.partialEval(v)
		hasUnknowns = hasUnknowns || itHasUnknowns
		if err != nil {
			return nil, hasUnknowns, err
		}
		tuple.Exprs[i] = asSyntax(newexpr)
	}
	return tuple, hasUnknowns, nil
}

func (c *Context) partialEvalObject(obj *hclsyntax.ObjectConsExpr) (hclsyntax.Expression, bool, error) {
	hasUnknowns := false
	for i, elem := range obj.Items {
		newkey, h1, err := c.partialEval(elem.KeyExpr)
		hasUnknowns = hasUnknowns || h1
		if err != nil {
			return nil, false, err
		}
		newval, h2, err := c.partialEval(elem.ValueExpr)
		hasUnknowns = hasUnknowns || h2
		if err != nil {
			return nil, false, err
		}
		elem.KeyExpr = asSyntax(newkey)
		elem.ValueExpr = asSyntax(newval)
		obj.Items[i] = elem
	}
	return obj, hasUnknowns, nil
}

func (c *Context) partialEvalFunc(funcall *hclsyntax.FunctionCallExpr) (hhcl.Expression, bool, error) {
	if strings.HasPrefix(funcall.Name, "tm_") {
		val, err := c.Eval(funcall)
		if err != nil {
			return nil, false, err
		}
		return &hclsyntax.LiteralValueExpr{
			Val:      val,
			SrcRange: funcall.Range(),
		}, false, nil
	}

	// hasUnknowns is true because the function is not tm_ prefixed

	for i, arg := range funcall.Args {
		newexpr, _, err := c.partialEval(arg)
		if err != nil {
			return nil, true, err
		}
		funcall.Args[i] = asSyntax(newexpr)
	}
	return funcall, true, nil
}

func (c *Context) partialEvalIndex(index *hclsyntax.IndexExpr) (hhcl.Expression, bool, error) {
	hasUnknowns := false
	newcol, h1, err := c.partialEval(index.Collection)
	if err != nil {
		return nil, false, err
	}
	index.Collection = asSyntax(newcol)
	hasUnknowns = h1
	newkey, h2, err := c.partialEval(index.Key)
	hasUnknowns = hasUnknowns || h2
	if err != nil {
		return nil, false, err
	}
	index.Key = asSyntax(newkey)
	if hasUnknowns {
		return index, hasUnknowns, nil
	}
	val, err := c.Eval(index)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: index.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalSplat(expr *hclsyntax.SplatExpr) (hhcl.Expression, bool, error) {
	newsrc, hasUnknowns, err := c.partialEval(expr.Source)
	if err != nil {
		return nil, false, err
	}
	expr.Source = asSyntax(newsrc)
	if hasUnknowns {
		return expr, hasUnknowns, nil
	}
	newEach, hasUnknowns, err := c.partialEval(expr.Each)
	if err != nil {
		return nil, false, err
	}
	expr.Each = asSyntax(newEach)
	if hasUnknowns {
		return expr, hasUnknowns, nil
	}
	val, err := c.Eval(expr)
	if err != nil {
		return expr, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: expr.Range(),
	}, false, nil
}

func (c *Context) partialEvalObjectKey(key *hclsyntax.ObjectConsKeyExpr) (hhcl.Expression, bool, error) {
	var (
		err         error
		wrapped     hhcl.Expression
		hasUnknowns bool
	)

	switch vexpr := key.Wrapped.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		wrapped, hasUnknowns, err = c.partialEvalScopeTrav(vexpr, partialEvalOption{
			// back-compatibility with old partial evaluator.
			// global = 1
			forbidRootEval: true,
		})
	case *hclsyntax.ParenthesesExpr:
		wrapped, hasUnknowns, err = c.partialEvalParenExpr(vexpr, partialEvalOption{
			// back-compatibility with old partial evaluator.
			// (global) = 1
			forbidRootEval: false,
		})
	default:
		wrapped, hasUnknowns, err = c.partialEval(key.Wrapped)
	}

	if err != nil {
		return nil, false, err
	}

	key.Wrapped = asSyntax(wrapped)
	return key, hasUnknowns, nil
}

func (c *Context) partialEvalForExpr(forExpr *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	resExpr, hasUnknowns, err := c.partialEval(forExpr.CollExpr)
	if err != nil {
		return nil, false, err
	}
	forExpr.CollExpr = asSyntax(resExpr)
	if !hasUnknowns {
		resExpr, hasUnknowns, err = c.evalForLoop(forExpr)
		if err != nil {
			return nil, false, err
		}
		if !hasUnknowns {
			return resExpr, false, nil
		}
	}

	if forExpr.KeyExpr != nil {
		resExpr, _, err = c.partialEval(forExpr.KeyExpr)
		if err != nil {
			return nil, false, err
		}
		forExpr.KeyExpr = asSyntax(resExpr)
	}

	if forExpr.ValExpr != nil {
		resExpr, _, err := c.partialEval(forExpr.ValExpr)
		if err != nil {
			return nil, false, err
		}
		forExpr.ValExpr = asSyntax(resExpr)
	}

	if forExpr.CondExpr != nil {
		resExpr, _, err := c.partialEval(forExpr.CondExpr)
		if err != nil {
			return nil, false, err
		}
		forExpr.CondExpr = asSyntax(resExpr)
	}

	return forExpr, true, nil
}

func (c *Context) evalForLoop(forExpr *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	if forExpr.KeyExpr != nil {
		return c.evalForObjectLoop(forExpr)
	}
	return c.evalForListLoop(forExpr)
}

func (c *Context) evalForObjectLoop(forExpr *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	res := make(map[string]cty.Value)
	coll, err := c.Eval(forExpr.CollExpr)
	if err != nil {
		return nil, false, err
	}
	if !coll.CanIterateElements() {
		return nil, false, errors.E("for-expr with non-iterable collection", forExpr.CollExpr.Range())
	}
	iterator := coll.ElementIterator()
	for iterator.Next() {
		k, v := iterator.Element()
		childCtx := c.ChildContext()
		childCtx.hclctx.Variables[forExpr.KeyVar] = k
		childCtx.hclctx.Variables[forExpr.ValVar] = v

		if forExpr.CondExpr != nil {
			condExpr, hasUnknowns, err := childCtx.partialEval(&ast.CloneExpression{
				Expression: forExpr.CondExpr,
			})
			if err != nil {
				return nil, false, err
			}
			if hasUnknowns {
				return forExpr, hasUnknowns, nil
			}
			condVal, err := childCtx.Eval(condExpr)
			if err != nil {
				return nil, false, err
			}
			if condVal.Type() != cty.Bool {
				return nil, false, errors.E("condition is not a boolean but %s", condVal.Type().FriendlyName())
			}
			if condVal.False() {
				continue
			}
		}
		resKeyExpr, hasUnknowns, err := childCtx.partialEval(&ast.CloneExpression{
			Expression: forExpr.KeyExpr,
		})
		if err != nil {
			return nil, false, err
		}

		if hasUnknowns {
			return forExpr, hasUnknowns, nil
		}

		resKeyVal, err := childCtx.Eval(resKeyExpr)
		if err != nil {
			return nil, false, err
		}
		if resKeyVal.Type() != cty.String {
			return nil, false, errors.E("object key must be a string", forExpr.KeyExpr.Range())
		}
		resValExpr, hasUnknowns, err := childCtx.partialEval(&ast.CloneExpression{
			Expression: forExpr.ValExpr,
		})
		if err != nil {
			return nil, false, err
		}
		if hasUnknowns {
			return forExpr, hasUnknowns, nil
		}
		resVal, err := childCtx.Eval(resValExpr)
		if err != nil {
			return nil, false, err
		}
		res[resKeyVal.AsString()] = resVal
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.ObjectVal(res),
		SrcRange: forExpr.SrcRange,
	}, false, nil
}

func (c *Context) evalForListLoop(forExpr *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	var res []cty.Value
	coll, err := c.Eval(forExpr.CollExpr)
	if err != nil {
		return nil, false, err
	}
	if !coll.CanIterateElements() {
		return nil, false, errors.E("for-expr with non-iterable collection", forExpr.CollExpr.Range())
	}
	iterator := coll.ElementIterator()
	for iterator.Next() {
		k, v := iterator.Element()
		childCtx := c.ChildContext()
		childCtx.hclctx.Variables[forExpr.KeyVar] = k
		childCtx.hclctx.Variables[forExpr.ValVar] = v

		if forExpr.CondExpr != nil {
			condExpr, hasUnknowns, err := childCtx.partialEval(&ast.CloneExpression{
				Expression: forExpr.CondExpr,
			})
			if err != nil {
				return nil, false, err
			}
			if hasUnknowns {
				return forExpr, hasUnknowns, nil
			}
			condVal, err := childCtx.Eval(condExpr)
			if err != nil {
				return nil, false, err
			}
			if condVal.Type() != cty.Bool {
				return nil, false, errors.E("condition is not a boolean but %s", condVal.Type().FriendlyName())
			}
			if condVal.False() {
				continue
			}
		}

		resValExpr, hasUnknowns, err := childCtx.partialEval(&ast.CloneExpression{
			Expression: forExpr.ValExpr,
		})
		if err != nil {
			return nil, false, err
		}
		if hasUnknowns {
			return forExpr, hasUnknowns, nil
		}
		resVal, err := childCtx.Eval(resValExpr)
		if err != nil {
			return nil, false, err
		}
		res = append(res, resVal)
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.TupleVal(res),
		SrcRange: forExpr.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalBinOp(binop *hclsyntax.BinaryOpExpr) (hhcl.Expression, bool, error) {
	lhs, h1, err := c.partialEval(binop.LHS)
	if err != nil {
		return nil, false, err
	}
	rhs, h2, err := c.partialEval(binop.RHS)
	if err != nil {
		return nil, false, err
	}
	binop.LHS = asSyntax(lhs)
	binop.RHS = asSyntax(rhs)
	return binop, h1 || h2, nil
}

func (c *Context) partialEvalUnaryOp(unary *hclsyntax.UnaryOpExpr) (hhcl.Expression, bool, error) {
	val, hasUnknowns, err := c.partialEval(unary.Val)
	if err != nil {
		return nil, false, err
	}
	unary.Val = asSyntax(val)
	return unary, hasUnknowns, nil
}

func (c *Context) partialEvalCondExpr(cond *hclsyntax.ConditionalExpr) (hhcl.Expression, bool, error) {
	newcond, h1, err := c.partialEval(cond.Condition)
	if err != nil {
		return nil, false, err
	}
	newtrue, h2, err := c.partialEval(cond.TrueResult)
	if err != nil {
		return nil, false, err
	}
	newfalse, h3, err := c.partialEval(cond.FalseResult)
	if err != nil {
		return nil, false, err
	}
	cond.Condition = asSyntax(newcond)
	cond.TrueResult = asSyntax(newtrue)
	cond.FalseResult = asSyntax(newfalse)
	return cond, h1 || h2 || h3, nil
}

type partialEvalOption struct {
	// if set to true, then root traversals like `global` or `iter` will not evaluate.
	forbidRootEval bool
}

func (c *Context) partialEvalScopeTrav(scope *hclsyntax.ScopeTraversalExpr, opts ...partialEvalOption) (hclsyntax.Expression, bool, error) {
	assertPartialExprOpt(opts)
	ns, ok := scope.Traversal[0].(hhcl.TraverseRoot)
	if !ok {
		return scope, false, nil
	}
	forbidRootEval := false
	if len(opts) == 1 {
		forbidRootEval = opts[0].forbidRootEval
	}
	if len(scope.Traversal) == 1 && forbidRootEval {
		return scope, false, nil
	}
	if !c.HasNamespace(ns.Name) {
		return scope, true, nil
	}
	val, err := c.Eval(scope)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: scope.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalParenExpr(paren *hclsyntax.ParenthesesExpr, opts ...partialEvalOption) (hhcl.Expression, bool, error) {
	assertPartialExprOpt(opts)
	forbidRootEval := false
	if len(opts) == 1 {
		forbidRootEval = opts[0].forbidRootEval
	}

	var (
		hasUnknowns bool
		newexpr     hhcl.Expression
		err         error
	)

	if scope, ok := paren.Expression.(*hclsyntax.ScopeTraversalExpr); ok && forbidRootEval {
		newexpr, hasUnknowns, err = c.partialEvalScopeTrav(scope, opts...)
	} else {
		newexpr, hasUnknowns, err = c.partialEval(paren.Expression)
	}

	if err != nil {
		return nil, false, err
	}

	paren.Expression = asSyntax(newexpr)
	return paren, hasUnknowns, nil
}

func (c *Context) partialEvalRelTrav(rel *hclsyntax.RelativeTraversalExpr) (hhcl.Expression, bool, error) {
	newsrc, hasUnknowns, err := c.partialEval(rel.Source)
	if err != nil {
		return nil, false, err
	}
	rel.Source = asSyntax(newsrc)
	if hasUnknowns {
		return rel, hasUnknowns, nil
	}

	val, err := c.Eval(rel)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: rel.SrcRange,
	}, false, nil
}

func asSyntax(expr hhcl.Expression) hclsyntax.Expression {
	return expr.(hclsyntax.Expression)
}

func assertPartialExprOpt(opts []partialEvalOption) {
	if len(opts) > 1 {
		panic(errors.E(errors.ErrInternal, "only 1 option object allowed in partialEvalOption"))
	}
}
