// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package eval

import (
	"fmt"
	"strings"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
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
	case *hclsyntax.LiteralValueExpr:
		return &hclsyntax.LiteralValueExpr{
			Val:      e.Val,
			SrcRange: e.SrcRange,
		}, false, nil
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

func (c *Context) partialEvalTemplate(old *hclsyntax.TemplateExpr) (*hclsyntax.TemplateExpr, bool, error) {
	new := &hclsyntax.TemplateExpr{SrcRange: old.SrcRange, Parts: make([]hclsyntax.Expression, len(old.Parts))}
	hasUnknowns := false
	for i, part := range old.Parts {
		newexpr, partHasUnknowns, err := c.partialEval(part)
		if err != nil {
			return nil, false, err
		}
		hasUnknowns = hasUnknowns || partHasUnknowns
		new.Parts[i] = asSyntax(newexpr)
	}
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalTmplWrap(old *hclsyntax.TemplateWrapExpr) (hhcl.Expression, bool, error) {
	newwrap, hasUnknowns, err := c.partialEval(old.Wrapped)
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
	new := &hclsyntax.TemplateWrapExpr{
		Wrapped:  asSyntax(newwrap),
		SrcRange: old.SrcRange,
	}
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalTuple(old *hclsyntax.TupleConsExpr) (hclsyntax.Expression, bool, error) {
	new := &hclsyntax.TupleConsExpr{
		Exprs:     make([]hclsyntax.Expression, len(old.Exprs)),
		SrcRange:  old.SrcRange,
		OpenRange: old.OpenRange,
	}
	hasUnknowns := false
	for i, v := range old.Exprs {
		newexpr, itHasUnknowns, err := c.partialEval(v)
		hasUnknowns = hasUnknowns || itHasUnknowns
		if err != nil {
			return nil, hasUnknowns, err
		}
		new.Exprs[i] = asSyntax(newexpr)
	}
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalObject(old *hclsyntax.ObjectConsExpr) (hclsyntax.Expression, bool, error) {
	new := &hclsyntax.ObjectConsExpr{
		SrcRange:  old.SrcRange,
		OpenRange: old.OpenRange,
		Items:     make([]hclsyntax.ObjectConsItem, len(old.Items)),
	}
	hasUnknowns := false
	for i, oldelem := range old.Items {
		// copy just in case elem is a pointer in the future
		newelem := hclsyntax.ObjectConsItem{}
		newkey, h1, err := c.partialEval(oldelem.KeyExpr)
		hasUnknowns = hasUnknowns || h1
		if err != nil {
			return nil, false, err
		}
		newval, h2, err := c.partialEval(oldelem.ValueExpr)
		hasUnknowns = hasUnknowns || h2
		if err != nil {
			return nil, false, err
		}
		newelem.KeyExpr = asSyntax(newkey)
		newelem.ValueExpr = asSyntax(newval)
		new.Items[i] = newelem
	}
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalFunc(old *hclsyntax.FunctionCallExpr) (hhcl.Expression, bool, error) {
	if strings.HasPrefix(old.Name, "tm_") {
		val, err := c.Eval(old)
		if err != nil {
			return nil, false, err
		}
		return &hclsyntax.LiteralValueExpr{
			Val:      val,
			SrcRange: old.Range(),
		}, false, nil
	}

	// hasUnknowns is true because the function is not tm_ prefixed
	new := &hclsyntax.FunctionCallExpr{
		Name:            old.Name,
		Args:            make([]hclsyntax.Expression, len(old.Args)),
		ExpandFinal:     old.ExpandFinal,
		NameRange:       old.NameRange,
		OpenParenRange:  old.OpenParenRange,
		CloseParenRange: old.CloseParenRange,
	}
	for i, oldarg := range old.Args {
		newexpr, _, err := c.partialEval(oldarg)
		if err != nil {
			return nil, true, err
		}
		new.Args[i] = asSyntax(newexpr)
	}
	return new, true, nil
}

func (c *Context) partialEvalIndex(old *hclsyntax.IndexExpr) (hhcl.Expression, bool, error) {
	hasUnknowns := false
	newcol, h1, err := c.partialEval(old.Collection)
	if err != nil {
		return nil, false, err
	}
	hasUnknowns = h1
	newkey, h2, err := c.partialEval(old.Key)
	hasUnknowns = hasUnknowns || h2
	if err != nil {
		return nil, false, err
	}
	if hasUnknowns {
		return &hclsyntax.IndexExpr{
			Key:          asSyntax(newkey),
			Collection:   asSyntax(newcol),
			SrcRange:     old.SrcRange,
			OpenRange:    old.OpenRange,
			BracketRange: old.BracketRange,
		}, hasUnknowns, nil
	}
	val, err := c.Eval(old)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: old.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalSplat(old *hclsyntax.SplatExpr) (hhcl.Expression, bool, error) {
	newsrc, hasUnknowns, err := c.partialEval(old.Source)
	if err != nil {
		return nil, false, err
	}
	new := &hclsyntax.SplatExpr{
		Each:        ast.CloneExpr(old.Each),
		Source:      asSyntax(newsrc),
		Item:        old.Item, // TODO(i4k): figure how to clone this!!
		SrcRange:    old.SrcRange,
		MarkerRange: old.MarkerRange,
	}
	if hasUnknowns {
		return new, hasUnknowns, nil
	}
	newEach, hasUnknowns, err := c.partialEval(new.Each)
	if err != nil {
		return nil, false, err
	}
	new.Each = asSyntax(newEach)
	if hasUnknowns {
		return new, hasUnknowns, nil
	}
	val, err := c.Eval(new)
	if err != nil {
		return new, false, err // TODO(i4k): why return new and err??
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: old.Range(),
	}, false, nil
}

func (c *Context) partialEvalObjectKey(old *hclsyntax.ObjectConsKeyExpr) (hhcl.Expression, bool, error) {
	var (
		err         error
		wrapped     hhcl.Expression
		hasUnknowns bool
	)

	switch vexpr := old.Wrapped.(type) {
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
		wrapped, hasUnknowns, err = c.partialEval(old.Wrapped)
	}

	if err != nil {
		return nil, false, err
	}

	return &hclsyntax.ObjectConsKeyExpr{
		ForceNonLiteral: old.ForceNonLiteral,
		Wrapped:         asSyntax(wrapped),
	}, hasUnknowns, nil
}

func (c *Context) partialEvalForExpr(old *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	resExpr, hasUnknowns, err := c.partialEval(old.CollExpr)
	if err != nil {
		return nil, false, err
	}
	new := &hclsyntax.ForExpr{
		CollExpr:   asSyntax(resExpr),
		KeyVar:     old.KeyVar,
		ValVar:     old.ValVar,
		KeyExpr:    ast.CloneExpr(old.KeyExpr),
		ValExpr:    ast.CloneExpr(old.ValExpr),
		CondExpr:   ast.CloneExpr(old.CondExpr),
		Group:      old.Group,
		SrcRange:   old.SrcRange,
		OpenRange:  old.OpenRange,
		CloseRange: old.CloseRange,
	}
	if !hasUnknowns {
		resExpr, hasUnknowns, err = c.evalForLoop(new)
		if err != nil {
			return nil, false, err
		}
		if !hasUnknowns {
			return resExpr, false, nil
		}
	}

	if new.KeyExpr != nil {
		resExpr, _, err = c.partialEval(new.KeyExpr)
		if err != nil {
			return nil, false, err
		}
		new.KeyExpr = asSyntax(resExpr)
	}

	if new.ValExpr != nil {
		resExpr, _, err := c.partialEval(new.ValExpr)
		if err != nil {
			return nil, false, err
		}
		new.ValExpr = asSyntax(resExpr)
	}

	if new.CondExpr != nil {
		resExpr, _, err := c.partialEval(new.CondExpr)
		if err != nil {
			return nil, false, err
		}
		new.CondExpr = asSyntax(resExpr)
	}

	return new, true, nil
}

func (c *Context) evalForLoop(old *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	if old.KeyExpr != nil {
		return c.evalForObjectLoop(old)
	}
	return c.evalForListLoop(old)
}

func (c *Context) evalForObjectLoop(old *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	res := make(map[string]cty.Value)
	newcoll, err := c.Eval(old.CollExpr)
	if err != nil {
		return nil, false, err
	}
	if !newcoll.CanIterateElements() {
		return nil, false, errors.E("for-expr with non-iterable collection", old.CollExpr.Range())
	}
	new := &hclsyntax.ForExpr{
		CollExpr:   ast.CloneExpr(old.CollExpr),
		KeyExpr:    ast.CloneExpr(old.KeyExpr),
		ValExpr:    ast.CloneExpr(old.ValExpr),
		CondExpr:   ast.CloneExpr(old.CondExpr),
		KeyVar:     old.KeyVar,
		ValVar:     old.ValVar,
		Group:      old.Group,
		SrcRange:   old.SrcRange,
		OpenRange:  old.OpenRange,
		CloseRange: old.CloseRange,
	}
	iterator := newcoll.ElementIterator()
	for iterator.Next() {
		k, v := iterator.Element()
		childCtx := c.ChildContext()
		childCtx.hclctx.Variables[old.KeyVar] = k
		childCtx.hclctx.Variables[old.ValVar] = v

		if old.CondExpr != nil {
			newCondExpr, hasUnknowns, err := childCtx.partialEval(new.CondExpr)
			if err != nil {
				return nil, false, err
			}
			if hasUnknowns {
				return new, hasUnknowns, nil
			}
			condVal, err := childCtx.Eval(newCondExpr)
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
		resKeyExpr, hasUnknowns, err := childCtx.partialEval(new.KeyExpr)
		if err != nil {
			return nil, false, err
		}

		if hasUnknowns {
			return new, hasUnknowns, nil
		}

		resKeyVal, err := childCtx.Eval(resKeyExpr)
		if err != nil {
			return nil, false, err
		}
		if resKeyVal.Type() != cty.String {
			return nil, false, errors.E("object key must be a string", old.KeyExpr.Range())
		}
		resValExpr, hasUnknowns, err := childCtx.partialEval(old.ValExpr)
		if err != nil {
			return nil, false, err
		}
		if hasUnknowns {
			return new, hasUnknowns, nil
		}
		resVal, err := childCtx.Eval(resValExpr)
		if err != nil {
			return nil, false, err
		}
		res[resKeyVal.AsString()] = resVal
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.ObjectVal(res),
		SrcRange: old.SrcRange,
	}, false, nil
}

func (c *Context) evalForListLoop(old *hclsyntax.ForExpr) (hhcl.Expression, bool, error) {
	var res []cty.Value
	newcoll, err := c.Eval(old.CollExpr)
	if err != nil {
		return nil, false, err
	}
	if !newcoll.CanIterateElements() {
		return nil, false, errors.E("for-expr with non-iterable collection", old.CollExpr.Range())
	}
	new := &hclsyntax.ForExpr{
		CollExpr:   ast.CloneExpr(old.CollExpr),
		KeyExpr:    ast.CloneExpr(old.KeyExpr),
		ValExpr:    ast.CloneExpr(old.ValExpr),
		CondExpr:   ast.CloneExpr(old.CondExpr),
		KeyVar:     old.KeyVar,
		ValVar:     old.ValVar,
		Group:      old.Group,
		SrcRange:   old.SrcRange,
		OpenRange:  old.OpenRange,
		CloseRange: old.CloseRange,
	}
	iterator := newcoll.ElementIterator()
	for iterator.Next() {
		k, v := iterator.Element()
		childCtx := c.ChildContext()
		childCtx.hclctx.Variables[old.KeyVar] = k
		childCtx.hclctx.Variables[old.ValVar] = v

		if old.CondExpr != nil {
			condExpr, hasUnknowns, err := childCtx.partialEval(new.CondExpr)
			if err != nil {
				return nil, false, err
			}
			if hasUnknowns {
				return new, hasUnknowns, nil
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

		resValExpr, hasUnknowns, err := childCtx.partialEval(new.ValExpr)
		if err != nil {
			return nil, false, err
		}
		if hasUnknowns {
			return new, hasUnknowns, nil
		}
		resVal, err := childCtx.Eval(resValExpr)
		if err != nil {
			return nil, false, err
		}
		res = append(res, resVal)
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      cty.TupleVal(res),
		SrcRange: new.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalBinOp(old *hclsyntax.BinaryOpExpr) (hhcl.Expression, bool, error) {
	lhs, h1, err := c.partialEval(old.LHS)
	if err != nil {
		return nil, false, err
	}
	rhs, h2, err := c.partialEval(old.RHS)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.BinaryOpExpr{
		LHS:      asSyntax(lhs),
		RHS:      asSyntax(rhs),
		Op:       old.Op, // not copied but this is never modified.
		SrcRange: old.SrcRange,
	}, h1 || h2, nil
}

func (c *Context) partialEvalUnaryOp(old *hclsyntax.UnaryOpExpr) (hhcl.Expression, bool, error) {
	val, hasUnknowns, err := c.partialEval(old.Val)
	if err != nil {
		return nil, false, err
	}
	new := &hclsyntax.UnaryOpExpr{SrcRange: old.SrcRange, SymbolRange: old.SymbolRange}
	new.Val = asSyntax(val)
	new.Op = old.Op
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalCondExpr(old *hclsyntax.ConditionalExpr) (hhcl.Expression, bool, error) {
	newcond, h1, err := c.partialEval(old.Condition)
	if err != nil {
		return nil, false, err
	}
	newtrue, h2, err := c.partialEval(old.TrueResult)
	if err != nil {
		return nil, false, err
	}
	newfalse, h3, err := c.partialEval(old.FalseResult)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.ConditionalExpr{
		SrcRange:    old.SrcRange,
		Condition:   asSyntax(newcond),
		TrueResult:  asSyntax(newtrue),
		FalseResult: asSyntax(newfalse),
	}, h1 || h2 || h3, nil
}

type partialEvalOption struct {
	// if set to true, then root traversals like `global` or `iter` will not evaluate.
	forbidRootEval bool
}

func (c *Context) partialEvalScopeTrav(old *hclsyntax.ScopeTraversalExpr, opts ...partialEvalOption) (hclsyntax.Expression, bool, error) {
	assertPartialExprOpt(opts)
	new := &hclsyntax.ScopeTraversalExpr{SrcRange: old.SrcRange}
	// shallow copy
	new.Traversal = make(hhcl.Traversal, len(old.Traversal))
	copy(new.Traversal, old.Traversal)
	old = nil

	ns, ok := new.Traversal[0].(hhcl.TraverseRoot)
	if !ok {
		return new, false, nil
	}
	forbidRootEval := false
	if len(opts) == 1 {
		forbidRootEval = opts[0].forbidRootEval
	}
	if len(new.Traversal) == 1 && forbidRootEval {
		return new, false, nil
	}
	if !c.HasNamespace(ns.Name) {
		return new, true, nil
	}
	val, err := c.Eval(new)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: new.SrcRange,
	}, false, nil
}

func (c *Context) partialEvalParenExpr(old *hclsyntax.ParenthesesExpr, opts ...partialEvalOption) (hhcl.Expression, bool, error) {
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

	if scope, ok := old.Expression.(*hclsyntax.ScopeTraversalExpr); ok && forbidRootEval {
		newexpr, hasUnknowns, err = c.partialEvalScopeTrav(scope, opts...)
	} else {
		newexpr, hasUnknowns, err = c.partialEval(old.Expression)
	}

	if err != nil {
		return nil, false, err
	}

	new := &hclsyntax.ParenthesesExpr{SrcRange: old.SrcRange}
	new.Expression = asSyntax(newexpr)
	return new, hasUnknowns, nil
}

func (c *Context) partialEvalRelTrav(old *hclsyntax.RelativeTraversalExpr) (hhcl.Expression, bool, error) {
	newsrc, hasUnknowns, err := c.partialEval(old.Source)
	if err != nil {
		return nil, false, err
	}
	new := &hclsyntax.RelativeTraversalExpr{SrcRange: old.SrcRange}
	new.Traversal = make(hhcl.Traversal, len(old.Traversal))
	copy(new.Traversal, old.Traversal)
	new.Source = asSyntax(newsrc)
	if hasUnknowns {
		return new, hasUnknowns, nil
	}
	val, err := c.Eval(old)
	if err != nil {
		return nil, false, err
	}
	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: new.SrcRange,
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
