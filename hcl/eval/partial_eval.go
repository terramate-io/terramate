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
	case *hclsyntax.TupleConsExpr:
		return expr, nil
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
	default:
		panic(fmt.Sprintf("no conversion for %T", expr))
	}
}
