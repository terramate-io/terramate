// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package mapexpr

import (
	"fmt"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

// MapExpr represents a `map` block.
type MapExpr struct {
	Origin   info.Range
	Attrs    Attributes
	Children []*MapExpr
}

// Attributes of the MapExpr block.
type Attributes struct {
	ForEach    hhcl.Expression
	Iterator   string
	Key        hhcl.Expression
	ValueAttr  hhcl.Expression
	ValueBlock *ast.MergedBlock
}

// NewMapExpr creates a new MapExpr instance.
func NewMapExpr(block *ast.MergedBlock) (*MapExpr, error) {
	children := []*MapExpr{}
	var valueBlock *ast.MergedBlock
	for _, subBlock := range block.Blocks {
		if valueBlock != nil {
			// the validation for multiple value blocks is done at the parser.
			panic(errors.E(errors.ErrInternal, "unexpected number of value blocks inside map"))
		}

		valueBlock = subBlock

		for _, childBlock := range valueBlock.Blocks {
			// child blocks are `map`.
			m, err := NewMapExpr(childBlock)
			if err != nil {
				return nil, errors.E(err, "creating nested `map` expression")
			}
			children = append(children, m)
		}
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
	if valueBlock == nil {
		// already validated, if no value block then a value attr must exist.
		valueExpr = block.Attributes["value"].Expr
	}

	return &MapExpr{
		Origin:   block.RawOrigins[0].Range,
		Children: children,
		Attrs: Attributes{
			ForEach:    block.Attributes["for_each"].Expr.(hclsyntax.Expression),
			Key:        block.Attributes["key"].Expr.(hclsyntax.Expression),
			ValueAttr:  valueExpr,
			ValueBlock: valueBlock,
			Iterator:   iterator,
		},
	}, nil
}

// Range of the map block.
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

// StartRange of the map block.
func (m *MapExpr) StartRange() hhcl.Range {
	return m.Range()
}

// Value evaluates the map block.
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
	foreach.ForEachElement(func(_, newElement cty.Value) (stop bool) {
		iteratorMap := map[string]cty.Value{
			"new": newElement,
		}

		evaluator.SetNamespace(m.Attrs.Iterator, iteratorMap)

		keyVal, err := evaluator.Eval(m.Attrs.Key)
		if err != nil {
			mapErr = errors.E(err, "failed to evaluate the map.key")
			return true
		}

		if keyVal.Type() != cty.String {
			mapErr = errors.E("map key is not a string but %s", keyVal.Type().FriendlyName())
			return true
		}

		oldElement, ok := objmap[keyVal.AsString()]
		if ok {
			iteratorMap["old"] = oldElement
			evaluator.SetNamespace(m.Attrs.Iterator, iteratorMap)
		}

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

				for _, subBlock := range m.Attrs.ValueBlock.Blocks {
					childEvaluator := evaluator.Copy()
					// only `map` block allowed inside `value` block.
					subMap, err := NewMapExpr(subBlock)
					if err != nil {
						mapErr = errors.E(err, "evaluating nested %q map block", subBlock.Labels[0])
						return true
					}
					val, diags := subMap.Value(childEvaluator.Unwrap())
					if diags.HasErrors() {
						mapErr = errors.E(diags, "evaluating nested %q map block", subBlock.Labels[0])
						return true
					}

					valueMap[subBlock.Labels[0]] = val
				}
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
				Severity: hhcl.DiagError,
				Summary:  "failed to evaluate map block",
				Detail:   mapErr.Error(),
				Subject:  m.Range().Ptr(),
			},
		}
	}

	evaluator.DeleteNamespace(m.Attrs.Iterator)
	return cty.ObjectVal(objmap), nil
}

// Variables returns the outer variables referenced by the map block.
// It ignores local scoped variables.
func (m *MapExpr) Variables() []hhcl.Traversal {
	allvars := []hhcl.Traversal{}

	appendVars := func(vars []hhcl.Traversal) {
		for _, v := range vars {
			if m.Attrs.Iterator != v.RootName() {
				allvars = append(allvars, v)
			}
		}
	}

	appendVars(m.Attrs.ForEach.Variables())
	appendVars(m.Attrs.Key.Variables())
	if m.Attrs.ValueAttr != nil {
		appendVars(m.Attrs.ValueAttr.Variables())
	}
	if m.Attrs.ValueBlock != nil {
		for _, attr := range m.Attrs.ValueBlock.Attributes.SortedList() {
			appendVars(attr.Expr.Variables())
		}

		for _, childMap := range m.Children {
			appendVars(childMap.Variables())
		}
	}
	return allvars
}
