// Copyright 2022 Mineiros GmbH
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
	foundValueBlock := false
	var valueBlock *ast.MergedBlock
	for _, subBlock := range block.Blocks {
		if foundValueBlock {
			// the validation for multiple value blocks is done at the parser.
			break
		}
		valueBlock = subBlock
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
	foreach.ForEachElement(func(key, newElement cty.Value) (stop bool) {
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

// Variables returns the variables referenced by the map block.
// TODO(i4k): implement.
func (m *MapExpr) Variables() []hhcl.Traversal {
	return nil
}
