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

func (c *Context) partialEval(expr hhcl.Expression) (newexpr hhcl.Expression, err error) {
	switch e := expr.(type) {
	case *ast.CloneExpression:
		cloned := ast.CloneExpr(e.Expression)
		return c.partialEval(cloned)
	case *hclsyntax.LiteralValueExpr:
		return expr, nil
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
	default:
		panic(fmt.Sprintf("not implemented %T", expr))
	}
}

func (c *Context) partialEvalTemplate(tmpl *hclsyntax.TemplateExpr) (*hclsyntax.TemplateExpr, error) {
	for i, part := range tmpl.Parts {
		newexpr, err := c.partialEval(part)
		if err != nil {
			return nil, err
		}
		tmpl.Parts[i] = asSyntax(newexpr)
	}
	return tmpl, nil
}

func (c *Context) partialEvalTmplWrap(wrap *hclsyntax.TemplateWrapExpr) (hhcl.Expression, error) {
	newwrap, err := c.partialEval(wrap.Wrapped)
	if err != nil {
		return nil, err
	}
	if v, ok := newwrap.(*hclsyntax.LiteralValueExpr); ok {
		// TODO(fix)
		if v.Val.Type() == cty.String && strings.Contains(v.Val.AsString(), "${") {
			panic(v.Val.AsString())
		}
		return v, nil
	}

	wrap.Wrapped = asSyntax(newwrap)
	return wrap, nil
}

func (c *Context) partialEvalTuple(tuple *hclsyntax.TupleConsExpr) (hclsyntax.Expression, error) {
	for i, v := range tuple.Exprs {
		newexpr, err := c.partialEval(v)
		if err != nil {
			return nil, err
		}
		tuple.Exprs[i] = asSyntax(newexpr)
	}
	return tuple, nil
}

func (c *Context) partialEvalObject(obj *hclsyntax.ObjectConsExpr) (hclsyntax.Expression, error) {
	for i, elem := range obj.Items {
		newkey, err := c.partialEval(elem.KeyExpr)
		if err != nil {
			return nil, err
		}
		newval, err := c.partialEval(elem.ValueExpr)
		if err != nil {
			return nil, err
		}
		elem.KeyExpr = asSyntax(newkey)
		elem.ValueExpr = asSyntax(newval)
		obj.Items[i] = elem
	}
	return obj, nil
}

func (c *Context) partialEvalFunc(funcall *hclsyntax.FunctionCallExpr) (hhcl.Expression, error) {
	if strings.HasPrefix(funcall.Name, "tm_") {
		val, err := c.Eval(funcall)
		if err != nil {
			return nil, err
		}
		return &hclsyntax.LiteralValueExpr{
			Val:      val,
			SrcRange: funcall.Range(),
		}, nil
	}
	for i, arg := range funcall.Args {
		newexpr, err := c.partialEval(arg)
		if err != nil {
			return nil, err
		}
		funcall.Args[i] = asSyntax(newexpr)
	}
	return funcall, nil
}

func (c *Context) partialEvalIndex(index *hclsyntax.IndexExpr) (hhcl.Expression, error) {
	newcol, err := c.partialEval(index.Collection)
	if err != nil {
		return nil, err
	}
	newkey, err := c.partialEval(index.Key)
	if err != nil {
		return nil, err
	}
	index.Collection = asSyntax(newcol)
	index.Key = asSyntax(newkey)

	if c.hasUnknownVars(index) {
		return index, nil
	}

	val, err := c.Eval(index)
	if err != nil {
		return nil, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: index.SrcRange,
	}, nil
}

func (c *Context) partialEvalSplat(expr *hclsyntax.SplatExpr) (hhcl.Expression, error) {
	newsrc, err := c.partialEval(expr.Source)
	if err != nil {
		return nil, err
	}
	expr.Source = asSyntax(newsrc)
	if c.hasUnknownVars(expr.Source) {
		return expr, nil
	}
	if c.hasUnknownVars(expr.Each) {
		return expr, nil
	}
	val, err := c.Eval(expr)
	if err != nil {
		// this can happen in the case of using funcalls not prefixed with tm_
		return expr, nil
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: expr.Range(),
	}, nil
}

func (c *Context) hasUnknownVars(expr hclsyntax.Expression) bool {
	for _, namespace := range expr.Variables() {
		if !c.HasNamespace(namespace.RootName()) {
			return true
		}
	}
	return false
}

func (c *Context) partialEvalObjectKey(key *hclsyntax.ObjectConsKeyExpr) (hhcl.Expression, error) {
	var (
		wrapped hhcl.Expression
		err     error
	)

	switch vexpr := key.Wrapped.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		wrapped, err = c.partialEvalScopeTrav(vexpr, partialEvalOption{
			// back-compatibility with old partial evaluator.
			// global = 1
			forbidRootEval: true,
		})
	case *hclsyntax.ParenthesesExpr:
		wrapped, err = c.partialEvalParenExpr(vexpr, partialEvalOption{
			// back-compatibility with old partial evaluator.
			// (global) = 1
			forbidRootEval: true,
		})
	default:
		wrapped, err = c.partialEval(key.Wrapped)
	}

	if err != nil {
		return nil, err
	}

	key.Wrapped = asSyntax(wrapped)
	return key, nil
}

func (c *Context) partialEvalForExpr(forExpr *hclsyntax.ForExpr) (hhcl.Expression, error) {
	resExpr, err := c.partialEval(forExpr.CollExpr)
	if err != nil {
		return nil, err
	}
	forExpr.CollExpr = asSyntax(resExpr)
	lit, canUnroll := forExpr.CollExpr.(*hclsyntax.LiteralValueExpr)
	if canUnroll && forExpr.CondExpr != nil {
		for _, traversal := range forExpr.CondExpr.Variables() {
			name := traversal.RootName()
			if name == forExpr.KeyVar ||
				name == forExpr.ValVar {
				continue
			}
			if _, ok := c.hclctx.Variables[name]; !ok {
				// cond depends on partial data
				canUnroll = false
				break
			}
		}
	}

	if canUnroll {
		return c.evalForLoop(lit, forExpr)
	}

	if forExpr.KeyExpr != nil {
		resExpr, err := c.partialEval(forExpr.KeyExpr)
		if err != nil {
			return nil, err
		}
		forExpr.KeyExpr = asSyntax(resExpr)
	}

	if forExpr.ValExpr != nil {
		resExpr, err := c.partialEval(forExpr.ValExpr)
		if err != nil {
			return nil, err
		}
		forExpr.ValExpr = asSyntax(resExpr)
	}

	if forExpr.CondExpr != nil {
		resExpr, err := c.partialEval(forExpr.CondExpr)
		if err != nil {
			return nil, err
		}
		forExpr.CondExpr = asSyntax(resExpr)
	}

	return forExpr, nil
}

func (c *Context) evalForLoop(coll *hclsyntax.LiteralValueExpr, forExpr *hclsyntax.ForExpr) (*hclsyntax.LiteralValueExpr, error) {
	if forExpr.KeyExpr != nil {
		return c.evalForObjectLoop(coll, forExpr)
	}
	return c.evalForListLoop(coll, forExpr)
}

func (c *Context) evalForObjectLoop(coll *hclsyntax.LiteralValueExpr, forExpr *hclsyntax.ForExpr) (*hclsyntax.LiteralValueExpr, error) {
	res := make(map[string]cty.Value)
	if !coll.Val.CanIterateElements() {
		return nil, errors.E("for-expr with non-iterable collection", coll.SrcRange)
	}
	iterator := coll.Val.ElementIterator()
	for iterator.Next() {
		k, v := iterator.Element()
		ctx := c.hclctx.NewChild()
		ctx.Variables = make(map[string]cty.Value)
		ctx.Variables[forExpr.KeyVar] = k
		ctx.Variables[forExpr.ValVar] = v

		if forExpr.CondExpr != nil {
			condVal, diags := forExpr.CondExpr.Value(ctx)
			if diags.HasErrors() {
				return nil, errors.E(diags)
			}
			if condVal.Type() != cty.Bool {
				return nil, errors.E("condition is not a boolean but %s", condVal.Type().FriendlyNameForConstraint())
			}
			if condVal.False() {
				continue
			}
		}
		resKey, diags := forExpr.KeyExpr.Value(ctx)
		if diags.HasErrors() {
			return nil, errors.E(diags)
		}
		if resKey.Type() != cty.String {
			return nil, errors.E("object key must be a string", forExpr.KeyExpr.Range())
		}
		resVal, diags := forExpr.ValExpr.Value(ctx)
		if diags.HasErrors() {
			return nil, errors.E(diags)
		}
		res[resKey.AsString()] = resVal
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.ObjectVal(res),
		SrcRange: forExpr.SrcRange,
	}, nil
}

func (c *Context) evalForListLoop(coll *hclsyntax.LiteralValueExpr, forExpr *hclsyntax.ForExpr) (*hclsyntax.LiteralValueExpr, error) {
	var res []cty.Value
	iterator := coll.Val.ElementIterator()

	for iterator.Next() {
		k, v := iterator.Element()
		ctx := c.hclctx.NewChild()
		ctx.Variables = make(map[string]cty.Value)
		ctx.Variables[forExpr.KeyVar] = k
		ctx.Variables[forExpr.ValVar] = v

		resVal, diags := forExpr.ValExpr.Value(ctx)
		if diags.HasErrors() {
			return nil, errors.E(diags)
		}
		res = append(res, resVal)
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.TupleVal(res),
		SrcRange: forExpr.SrcRange,
	}, nil
}

func (c *Context) partialEvalBinOp(binop *hclsyntax.BinaryOpExpr) (hhcl.Expression, error) {
	lhs, err := c.partialEval(binop.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := c.partialEval(binop.RHS)
	if err != nil {
		return nil, err
	}
	binop.LHS = asSyntax(lhs)
	binop.RHS = asSyntax(rhs)
	return binop, nil
}

func (c *Context) partialEvalUnaryOp(unary *hclsyntax.UnaryOpExpr) (hhcl.Expression, error) {
	val, err := c.partialEval(unary.Val)
	if err != nil {
		return nil, err
	}
	unary.Val = asSyntax(val)
	return unary, nil
}

func (c *Context) partialEvalCondExpr(cond *hclsyntax.ConditionalExpr) (hhcl.Expression, error) {
	newcond, err := c.partialEval(cond.Condition)
	if err != nil {
		return nil, err
	}
	newtrue, err := c.partialEval(cond.TrueResult)
	if err != nil {
		return nil, err
	}
	newfalse, err := c.partialEval(cond.FalseResult)
	if err != nil {
		return nil, err
	}
	cond.Condition = asSyntax(newcond)
	cond.TrueResult = asSyntax(newtrue)
	cond.FalseResult = asSyntax(newfalse)
	return cond, nil
}

type partialEvalOption struct {
	// if set to true, then root traversals like `global` or `iter` will not evaluate.
	forbidRootEval bool
}

func (c *Context) partialEvalScopeTrav(scope *hclsyntax.ScopeTraversalExpr, opts ...partialEvalOption) (hclsyntax.Expression, error) {
	assertPartialExprOpt(opts)
	ns, ok := scope.Traversal[0].(hhcl.TraverseRoot)
	if !ok {
		return scope, nil
	}
	if !c.HasNamespace(ns.Name) {
		return scope, nil
	}
	forbidRootEval := false
	if len(opts) == 1 {
		forbidRootEval = opts[0].forbidRootEval
	}
	if len(scope.Traversal) == 1 && forbidRootEval {
		return scope, nil
	}

	val, err := c.Eval(scope)
	if err != nil {
		return nil, err
	}

	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: scope.SrcRange,
	}, nil
}

func (c *Context) partialEvalParenExpr(paren *hclsyntax.ParenthesesExpr, opts ...partialEvalOption) (hhcl.Expression, error) {
	assertPartialExprOpt(opts)
	forbidRootEval := false
	if len(opts) == 1 {
		forbidRootEval = opts[0].forbidRootEval
	}

	var (
		newexpr hhcl.Expression
		err     error
	)

	if scope, ok := paren.Expression.(*hclsyntax.ScopeTraversalExpr); ok && forbidRootEval {
		newexpr, err = c.partialEvalScopeTrav(scope, opts...)
	} else {
		newexpr, err = c.partialEval(paren.Expression)
	}

	if err != nil {
		return nil, err
	}

	paren.Expression = asSyntax(newexpr)
	return paren, nil
}

func (c *Context) partialEvalRelTrav(rel *hclsyntax.RelativeTraversalExpr) (hhcl.Expression, error) {
	newsrc, err := c.partialEval(rel.Source)
	if err != nil {
		return nil, err
	}
	rel.Source = asSyntax(newsrc)
	if c.hasUnknownVars(rel) {
		return rel, nil
	}

	val, err := c.Eval(rel)
	if err != nil {
		return nil, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: rel.SrcRange,
	}, nil
}

func asSyntax(expr hhcl.Expression) hclsyntax.Expression {
	return expr.(hclsyntax.Expression)
}

func assertPartialExprOpt(opts []partialEvalOption) {
	if len(opts) > 1 {
		panic(errors.E(errors.ErrInternal, "only 1 option object allowed in partialEvalOption"))
	}
}
