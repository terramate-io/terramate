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

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
)

func (c *Context) partialEval(expr hhcl.Expression) (hhcl.Expression, error) {
	switch e := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return expr, nil
	case *hclsyntax.BinaryOpExpr:
		return c.partialEvalBinOp(e)
	case *hclsyntax.TupleConsExpr:
		return c.partialEvalTuple(e)
	case *hclsyntax.ObjectConsExpr:
		return c.partialEvalObject(e)
	case *hclsyntax.ObjectConsKeyExpr:
		return e, nil
	case *hclsyntax.TemplateExpr:
		return c.partialEvalTemplate(e)
	case *hclsyntax.ScopeTraversalExpr:
		return c.partialEvalScopeTrav(e)
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

func (c *Context) partialEvalScopeTrav(scope *hclsyntax.ScopeTraversalExpr) (hclsyntax.Expression, error) {
	ns, ok := scope.Traversal[0].(hhcl.TraverseRoot)
	if !ok {
		return scope, nil
	}
	if ns.Name != "global" && ns.Name != "terramate" {
		return scope, nil
	}
	val, diags := scope.Value(c.hclctx)
	if diags.HasErrors() {
		return nil, errors.E(diags, "evaluating %s", ast.TokensForExpression(scope).Bytes())
	}

	return &hclsyntax.LiteralValueExpr{
		Val:      val,
		SrcRange: scope.SrcRange,
	}, nil
}

func asSyntax(expr hhcl.Expression) hclsyntax.Expression {
	switch v := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		return v
	case *hclsyntax.TemplateExpr:
		return v
	case *hclsyntax.ScopeTraversalExpr:
		return v
	case *hclsyntax.TupleConsExpr:
		return v
	case *hclsyntax.ObjectConsExpr:
		return v
	case *hclsyntax.ObjectConsKeyExpr:
		return v
	case *hclsyntax.BinaryOpExpr:
		return v
	default:
		panic(fmt.Sprintf("no conversion for %T", expr))
	}
}
