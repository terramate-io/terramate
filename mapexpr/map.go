package mapexpr

import (
	"fmt"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

type MapExpr struct {
	Origin   info.Range
	Attrs    Attributes
	Children []*MapExpr
}

type Attributes struct {
	ForEach    hhcl.Expression
	Iterator   string
	Key        hhcl.Expression
	ValueAttr  hhcl.Expression
	ValueBlock *ast.Block
}

func NewMapExpr(block *ast.MergedBlock) (*MapExpr, error) {
	var children []*MapExpr
	foundValueBlock := false
	var valueBlock *ast.Block
	for _, subBlock := range block.Blocks {
		if foundValueBlock {
			// the validation for multiple value blocks is done at the parser.
			break
		}
		valueBlock = subBlock.RawOrigins[0]
		foundValueBlock = true
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

	var valueExpr hhcl.Expression
	if !foundValueBlock {
		// already validated, if no value block then a value attr must exist.
		valueExpr = block.Attributes["value"].Expr
	}

	return &MapExpr{
		Origin: block.RawOrigins[0].Range,
		Attrs: Attributes{
			ForEach:    block.Attributes["for_each"].Expr,
			Key:        block.Attributes["key"].Expr,
			ValueAttr:  valueExpr,
			ValueBlock: valueBlock,
			Iterator:   iterator,
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
	foreach, diags := m.Attrs.ForEach.Value(ctx)
	if diags.HasErrors() {
		return foreach, diags
	}

	if !foreach.CanIterateElements() {
		return cty.NilVal, hhcl.Diagnostics{
			&hhcl.Diagnostic{
				Summary: fmt.Sprintf("`for_each` expression of type %s cannot be iterated",
					foreach.Type().FriendlyName()),
				Subject: m.Attrs.ForEach.Range().Ptr(),
			},
		}
	}

	objmap := map[string]cty.Value{}
	evaluator := eval.NewContextFrom(ctx)

	var mapErr error
	foreach.ForEachElement(func(key, value cty.Value) (stop bool) {
		evaluator.SetNamespace(m.Attrs.Iterator, map[string]cty.Value{
			"new": value,
			"old": cty.NilVal,
		})

		keyVal, err := evaluator.Eval(m.Attrs.Key)
		if err != nil {
			mapErr = err
			return true
		}

		if keyVal.Type() != cty.String {
			mapErr = errors.E("map key is not a string but %s", keyVal.Type().FriendlyName())
			return true
		}

		old, ok := objmap[keyVal.AsString()]
		if !ok {
			old = cty.NilVal
		}
		evaluator.SetNamespace(m.Attrs.Iterator, map[string]cty.Value{
			"new": value,
			"old": old,
		})

		var valVal cty.Value

		if m.Attrs.ValueBlock != nil {
			valueMap := map[string]cty.Value{}

			for _, attr := range m.Attrs.ValueBlock.Attributes.SortedList() {
				attrVal, err := evaluator.Eval(attr.Expr)
				if err != nil {
					mapErr = err
					return true
				}

				valueMap[attr.Name] = attrVal
			}

			valVal = cty.ObjectVal(valueMap)
		} else {
			valVal, err = evaluator.Eval(m.Attrs.ValueAttr)
			if err != nil {
				mapErr = errors.E(err, "failed to evaluate map.value")
				return true
			}
		}

		objmap[keyVal.AsString()] = valVal
		return false
	})

	if mapErr != nil {
		return cty.NilVal, hhcl.Diagnostics{
			&hhcl.Diagnostic{
				Summary: "failed to evaluate map block",
				Detail:  mapErr.Error(),
				Subject: m.Range().Ptr(),
			},
		}
	}

	evaluator.DeleteNamespace(m.Attrs.Iterator)
	return cty.ObjectVal(objmap), nil
}

func (m *MapExpr) Variables() []hhcl.Traversal {
	return nil
}
