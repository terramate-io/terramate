// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"github.com/terramate-io/hcl/v2/hclparse"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

const (
	errHCLEncode errors.Kind = "failed to HCL encode the value"
	errHCLDecode errors.Kind = "failed to HCL decode content"
)

// HCLEncode implements the `tm_hclencode()` function.
func HCLEncode() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "val",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return hclEncode(args[0])
		},
	})
}

// HCLDecode implements the `tm_hcldecode` function.
func HCLDecode() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "content",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			return hclDecode(args[0].AsString())
		},
	})
}

func hclEncode(obj cty.Value) (cty.Value, error) {
	if !obj.Type().IsObjectType() && !obj.Type().IsMapType() {
		return cty.NilVal, errors.E(errHCLEncode, "only object/map can be encoded but got %s", obj.Type().FriendlyName())
	}
	out := hclwrite.NewFile()
	body := out.Body()
	it := obj.ElementIterator()
	for it.Next() {
		key, val := it.Element()
		if !key.Type().Equals(cty.String) {
			return cty.NilVal, errors.E(errHCLEncode, "top-level object key is not a string but %s", key.Type().FriendlyName())
		}
		attrName := key.AsString()
		if !hclsyntax.ValidIdentifier(attrName) {
			return cty.NilVal, errors.E(errHCLEncode, "top-level object key is not a valid HCL attribute name")
		}
		body.SetAttributeValue(attrName, val)
	}
	res, err := fmt.FormatMultiline(string(out.Bytes()), "<out>")
	if err != nil {
		return cty.NilVal, errors.E(errHCLEncode, err)
	}
	return cty.StringVal(res), nil
}

func hclDecode(content string) (cty.Value, error) {
	parser := hclparse.NewParser()
	file, err := parser.ParseHCL([]byte(content), "<input>")
	if err != nil {
		return cty.NilVal, errors.E(err, errHCLDecode)
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		panic(errors.E(errors.ErrInternal, "unexpected body type, please report this bug"))
	}
	errs := errors.L()
	for _, block := range body.Blocks {
		errs.Append(errors.E("tm_hcldecode() does not support blocks", block.Range()))
	}

	if err := errs.AsError(); err != nil {
		return cty.NilVal, errors.E(err, errHCLDecode)
	}

	attrs := map[string]cty.Value{}
	for _, attr := range ast.SortRawAttributes(ast.AsHCLAttributes(body.Attributes)) {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags, "failed to evaluate attribute %q", attr.Name))
		} else {
			attrs[attr.Name] = val
		}
	}
	if err := errs.AsError(); err != nil {
		return cty.NilVal, errors.E(errHCLDecode, err)
	}
	return cty.ObjectVal(attrs), nil
}
