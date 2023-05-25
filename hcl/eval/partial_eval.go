// Copyright 2023 Mineiros GmbH
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
	ErrPartial             errors.Kind = "partial evaluation failed"
	ErrInterpolation       errors.Kind = "interpolation failed"
	ErrForExprDisallowEval errors.Kind = "`for` expression disallow globals/terramate variables"
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

func (c *Context) hasTerramateVars(expr hclsyntax.Expression) bool {
	for _, namespace := range expr.Variables() {
		if c.HasNamespace(namespace.RootName()) {
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
	for _, expr := range []hclsyntax.Expression{
		forExpr.KeyExpr,
		forExpr.ValExpr,
		forExpr.CollExpr,
		forExpr.CondExpr,
	} {
		if expr != nil && c.hasTerramateVars(forExpr.CollExpr) {
			return nil, errors.E(ErrForExprDisallowEval, "evaluating expression: %s", ast.TokensForExpression(forExpr.CollExpr).Bytes())
		}
	}

	return forExpr, nil
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
