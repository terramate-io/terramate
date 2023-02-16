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

package ast

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// CloneExpression is an expression wrapper that
type CloneExpression struct {
	hclsyntax.Expression
}

// Value evaluates the wrapped expression.
func (clone *CloneExpression) Value(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	return clone.Expression.Value(ctx)
}

// CloneExpr clones the given expression.
func CloneExpr(expr hclsyntax.Expression) hclsyntax.Expression {
	if expr == nil {
		// for readability of this function we dont if-else against nil
		// expressions in object fields, so we bail out here.
		return expr
	}
	switch e := expr.(type) {
	case *CloneExpression:
		return &CloneExpression{
			Expression: CloneExpr(e.Expression),
		}
	case *hclsyntax.LiteralValueExpr:
		return &hclsyntax.LiteralValueExpr{
			Val:      e.Val,
			SrcRange: e.SrcRange,
		}
	case *hclsyntax.TemplateExpr:
		parts := make([]hclsyntax.Expression, len(e.Parts))
		for i, part := range e.Parts {
			parts[i] = CloneExpr(part)
		}
		return &hclsyntax.TemplateExpr{
			Parts:    parts,
			SrcRange: e.SrcRange,
		}
	case *hclsyntax.TemplateWrapExpr:
		return &hclsyntax.TemplateWrapExpr{
			Wrapped:  CloneExpr(e.Wrapped),
			SrcRange: e.SrcRange,
		}
	case *hclsyntax.BinaryOpExpr:
		return &hclsyntax.BinaryOpExpr{
			LHS:      CloneExpr(e.LHS),
			Op:       e.Op,
			RHS:      CloneExpr(e.RHS),
			SrcRange: e.SrcRange,
		}
	case *hclsyntax.UnaryOpExpr:
		return &hclsyntax.UnaryOpExpr{
			Op:       e.Op,
			Val:      CloneExpr(e.Val),
			SrcRange: e.SrcRange,
		}
	case *hclsyntax.TupleConsExpr:
		exprs := make([]hclsyntax.Expression, len(e.Exprs))
		for i, expr := range e.Exprs {
			exprs[i] = CloneExpr(expr)
		}
		return &hclsyntax.TupleConsExpr{
			Exprs:     exprs,
			SrcRange:  e.SrcRange,
			OpenRange: e.OpenRange,
		}
	case *hclsyntax.ParenthesesExpr:
		return &hclsyntax.ParenthesesExpr{
			Expression: CloneExpr(e.Expression),
			SrcRange:   e.SrcRange,
		}
	case *hclsyntax.ObjectConsExpr:
		items := make([]hclsyntax.ObjectConsItem, len(e.Items))
		for i, item := range e.Items {
			items[i] = hclsyntax.ObjectConsItem{
				KeyExpr:   CloneExpr(item.KeyExpr),
				ValueExpr: CloneExpr(item.ValueExpr),
			}
		}
		return &hclsyntax.ObjectConsExpr{
			Items:     items,
			SrcRange:  e.SrcRange,
			OpenRange: e.OpenRange,
		}
	case *hclsyntax.ObjectConsKeyExpr:
		return &hclsyntax.ObjectConsKeyExpr{
			Wrapped:         CloneExpr(e.Wrapped),
			ForceNonLiteral: e.ForceNonLiteral,
		}
	case *hclsyntax.ScopeTraversalExpr:
		return &hclsyntax.ScopeTraversalExpr{
			Traversal: e.Traversal,
			SrcRange:  e.SrcRange,
		}
	case *hclsyntax.ConditionalExpr:
		return &hclsyntax.ConditionalExpr{
			Condition:   CloneExpr(e.Condition),
			TrueResult:  CloneExpr(e.TrueResult),
			FalseResult: CloneExpr(e.FalseResult),
			SrcRange:    e.SrcRange,
		}
	case *hclsyntax.FunctionCallExpr:
		args := make([]hclsyntax.Expression, len(e.Args))
		for i, arg := range e.Args {
			args[i] = CloneExpr(arg)
		}
		return &hclsyntax.FunctionCallExpr{
			Name:            e.Name,
			Args:            args,
			ExpandFinal:     e.ExpandFinal,
			NameRange:       e.NameRange,
			OpenParenRange:  e.OpenParenRange,
			CloseParenRange: e.CloseParenRange,
		}
	case *hclsyntax.IndexExpr:
		return &hclsyntax.IndexExpr{
			Collection:   CloneExpr(e.Collection),
			Key:          CloneExpr(e.Key),
			SrcRange:     e.SrcRange,
			OpenRange:    e.OpenRange,
			BracketRange: e.BracketRange,
		}
	case *hclsyntax.ForExpr:
		return &hclsyntax.ForExpr{
			KeyVar:   e.KeyVar,
			ValVar:   e.ValVar,
			CollExpr: CloneExpr(e.CollExpr),
			KeyExpr:  CloneExpr(e.KeyExpr),
			ValExpr:  CloneExpr(e.ValExpr),
			CondExpr: CloneExpr(e.CondExpr),
			Group:    e.Group,
		}
	case *hclsyntax.SplatExpr:
		return &hclsyntax.SplatExpr{
			Source:      CloneExpr(e.Source),
			Each:        CloneExpr(e.Each),
			Item:        e.Item,
			SrcRange:    e.SrcRange,
			MarkerRange: e.MarkerRange,
		}
	case *hclsyntax.AnonSymbolExpr:
		return e
	case *hclsyntax.RelativeTraversalExpr:
		traversals := make(hcl.Traversal, len(e.Traversal))
		copy(traversals, e.Traversal)
		return &hclsyntax.RelativeTraversalExpr{
			Source:    CloneExpr(e.Source),
			Traversal: traversals,
		}
	default:
		panic(fmt.Sprintf("type %T not supported\n", e))
	}
}
