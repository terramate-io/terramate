// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml

import (
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/ext/customdecode"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

// ConvertToHCL transforms decoded YAML any values (as found in Map and Seq)
// into HCL expressions. Supported types are:
// - Built-in Go types that are produced by the default go-yaml decoder
// - HCL expressions -> passed through
// - Seq -> transformed to HCL tuple
// - Map -> transformed to HCL object
func ConvertToHCL(in any, srcRange hcl.Range) (hclsyntax.Expression, error) {
	switch v := in.(type) {
	case nil:
		return &hclsyntax.LiteralValueExpr{Val: cty.NullVal(cty.DynamicPseudoType), SrcRange: srcRange}, nil
	case string:
		return &hclsyntax.LiteralValueExpr{Val: cty.StringVal(v), SrcRange: srcRange}, nil
	case bool:
		return &hclsyntax.LiteralValueExpr{Val: cty.BoolVal(v), SrcRange: srcRange}, nil
	case int:
		return &hclsyntax.LiteralValueExpr{Val: cty.NumberIntVal(int64(v)), SrcRange: srcRange}, nil
	case float64:
		return &hclsyntax.LiteralValueExpr{Val: cty.NumberFloatVal(v), SrcRange: srcRange}, nil
	case Seq[any]:
		exprs := make([]hclsyntax.Expression, len(v))
		for i, elem := range v {
			seqExpr, err := ConvertToHCL(elem.Value, srcRange)
			if err != nil {
				return nil, err
			}
			exprs[i] = seqExpr
		}
		return &hclsyntax.TupleConsExpr{Exprs: exprs}, nil
	case Map[any]:
		items := make([]hclsyntax.ObjectConsItem, len(v))
		for i, elem := range v {
			keyExpr, err := ConvertToHCL(elem.Key, srcRange)
			if err != nil {
				return nil, err
			}
			valExpr, err := ConvertToHCL(elem.Value, srcRange)
			if err != nil {
				return nil, err
			}
			items[i] = hclsyntax.ObjectConsItem{
				KeyExpr:   keyExpr,
				ValueExpr: valExpr,
			}
		}
		return &hclsyntax.ObjectConsExpr{Items: items, SrcRange: srcRange}, nil
	case hclsyntax.Expression:
		return v, nil
	case interface{ unwrapAttribute() (any, int, int) }:
		unwrappedV, line, column := v.unwrapAttribute()
		newRange := hcl.Range{
			Filename: srcRange.Filename,
			Start:    hcl.Pos{Line: line, Column: column},
		}
		return ConvertToHCL(unwrappedV, newRange)
	}
	return nil, errors.E("unsupported YAML expression %T", in)
}

// ConvertFromCty transforms a cty.Value to a YAML any.
// Primitive types are converted to their respective Go type.
// HCL expressions are passed through.
// Object-like types are converted to Map.
// List-like types are converted to Seq.
func ConvertFromCty(val cty.Value) (any, error) {
	switch typ := val.Type(); {
	case typ == customdecode.ExpressionClosureType:
		closureExpr := val.EncapsulatedValue().(*customdecode.ExpressionClosure)
		return closureExpr.Expression, nil
	case typ == customdecode.ExpressionType:
		return customdecode.ExpressionFromVal(val), nil
	case val.IsNull():
		return nil, nil
	case typ == cty.Bool:
		return val.True(), nil
	case typ == cty.Number:
		n, _ := val.AsBigFloat().Float64()
		return n, nil
	case typ == cty.String:
		return val.AsString(), nil
	case typ.IsMapType() || typ.IsObjectType():
		m := make(Map[any], 0, val.LengthInt())
		for it := val.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			v, err := ConvertFromCty(ev)
			if err != nil {
				return nil, err
			}
			m = append(m, MapItem[any]{
				Key:   Attr(k.AsString()),
				Value: Attr(v),
			})
		}
		return m, nil
	case typ.IsListType() || typ.IsSetType() || typ.IsTupleType():
		m := make(Seq[any], 0, val.LengthInt())
		for it := val.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			v, err := ConvertFromCty(ev)
			if err != nil {
				return nil, err
			}
			m = append(m, SeqItem[any]{
				Value: Attr(v),
			})
		}
		return m, nil
	default:
		return nil, errors.E(errors.ErrInternal, "formatting for value type %s is not supported", val.Type().FriendlyName())
	}
}
