package mapexpr

import (
	"fmt"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

type MapExpr struct {
	Origin   info.Range
	Attrs    Attributes
	Children []*MapExpr
}

type Attributes struct {
	ForEach  hhcl.Expression
	Iterator string
	Key      hhcl.Expression
	Value    hhcl.Expression
}

func NewMapExpr(block *ast.MergedBlock) (*MapExpr, error) {
	var children []*MapExpr
	for _, subBlock := range block.Blocks {
		m, err := NewMapExpr(subBlock)
		if err != nil {
			return nil, err
		}
		children = append(children, m)
	}

	iterator := "element"
	if it, ok := block.Attributes["iterator"]; ok {
		iteratorTraversal, diags := hhcl.AbsTraversalForExpr(it.Expr)
		if diags.HasErrors() {
			return nil, errors.E(diags)
		}
		if len(iteratorTraversal) != 1 {
			return nil, errors.E(
				it.Range,
				"dynamic iterator must be a single variable name",
			)
		}
		iterator = iteratorTraversal.RootName()
	}

	return &MapExpr{
		Origin: block.RawOrigins[0].Range,
		Attrs: Attributes{
			ForEach:  block.Attributes["for_each"].Expr,
			Key:      block.Attributes["key"].Expr,
			Value:    block.Attributes["value"].Expr,
			Iterator: iterator,
		},
		Children: children,
	}, nil
}

func (m *MapExpr) Range() hhcl.Range {
	return hhcl.Range{
		Filename: m.Origin.HostPath(),
		Start: hhcl.Pos{
			Byte:   m.Origin.Start().Byte(),
			Column: m.Origin.Start().Column(),
			Line:   m.Origin.Start().Line(),
		},
		End: hhcl.Pos{
			Byte:   m.Origin.End().Byte(),
			Column: m.Origin.End().Column(),
			Line:   m.Origin.End().Line(),
		},
	}
}

func (m *MapExpr) StartRange() hhcl.Range {
	return m.Range()
}

func (m *MapExpr) Value(ctx *hhcl.EvalContext) (cty.Value, hhcl.Diagnostics) {
	forEach, diags := m.Attrs.ForEach.Value(ctx)
	if diags.HasErrors() {
		return forEach, diags
	}

	if !forEach.CanIterateElements() {
		return cty.NilVal, hhcl.Diagnostics{
			&hhcl.Diagnostic{
				Summary: fmt.Sprintf("`for_each` expression of type %s cannot be iterated",
					forEach.Type().FriendlyName()),
				Subject: m.Attrs.ForEach.Range().Ptr(),
			},
		}
	}
	return cty.NilVal, nil
}

func (m *MapExpr) Variables() []hhcl.Traversal {
	return nil
}
