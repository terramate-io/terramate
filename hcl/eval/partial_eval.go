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
)

func (c *Context) partialEval(expr hhcl.Expression) (hhcl.Expression, error) {
	switch e := expr.(type) {
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
	case *hclsyntax.ForExpr:
		return c.partialEvalForExpr(e)
	case *hclsyntax.ObjectConsKeyExpr:
		return e, nil
	case *hclsyntax.TemplateExpr:
		return c.partialEvalTemplate(e)
	case *hclsyntax.ScopeTraversalExpr:
		return c.partialEvalScopeTrav(e)
	case *hclsyntax.RelativeTraversalExpr:
		return c.partialEvalRelTrav(e)
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
	return index, nil
}

func (c *Context) partialEvalForExpr(forExpr *hclsyntax.ForExpr) (hhcl.Expression, error) {
	newcol, err := c.partialEval(forExpr.CollExpr)
	if err != nil {
		return nil, err
	}

	forExpr.CollExpr = asSyntax(newcol)

	// TODO(i4k): return ErrForExprDisallowEval in the case that Terramate
	// variables or funcalls are used in the keyExpr, valExpr or condExpr.

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

func (c *Context) partialEvalScopeTrav(scope *hclsyntax.ScopeTraversalExpr) (hclsyntax.Expression, error) {
	ns, ok := scope.Traversal[0].(hhcl.TraverseRoot)
	if !ok {
		return scope, nil
	}
	if !c.HasNamespace(ns.Name) {
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

func (c *Context) partialEvalRelTrav(rel *hclsyntax.RelativeTraversalExpr) (hhcl.Expression, error) {
	newsrc, err := c.partialEval(rel.Source)
	if err != nil {
		return nil, err
	}
	rel.Source = asSyntax(newsrc)
	return rel, nil
}

func asSyntax(expr hhcl.Expression) hclsyntax.Expression {
	switch v := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return v
	case *hclsyntax.TemplateExpr:
		return v
	case *hclsyntax.ScopeTraversalExpr:
		return v
	case *hclsyntax.RelativeTraversalExpr:
		return v
	case *hclsyntax.TupleConsExpr:
		return v
	case *hclsyntax.ObjectConsExpr:
		return v
	case *hclsyntax.ObjectConsKeyExpr:
		return v
	case *hclsyntax.FunctionCallExpr:
		return v
	case *hclsyntax.ForExpr:
		return v
	case *hclsyntax.BinaryOpExpr:
		return v
	case *hclsyntax.UnaryOpExpr:
		return v
	case *hclsyntax.ConditionalExpr:
		return v
	case *hclsyntax.IndexExpr:
		return v
	default:
		panic(fmt.Sprintf("no conversion for %T", expr))
	}
}
