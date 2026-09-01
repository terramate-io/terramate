// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"strings"

	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/typeschema"
)

// EvalUsesSchemas evaluates a uses_schemas block and returns the resolved schemas.
func EvalUsesSchemas(root *Root, resolveAPI resolve.API, evalctx *eval.Context, usesSchemasHCL *hcl.UsesSchemas, allowFetch bool) ([]*typeschema.Schema, error) {
	src, err := EvalString(evalctx, usesSchemasHCL.Source.Expr, "source")
	if err != nil {
		return nil, err
	}

	resolvedSrc, err := resolveAPI.Resolve(root.HostDir(), src, resolve.Schema, allowFetch)
	if err != nil {
		return nil, errors.E(usesSchemasHCL.Source.Expr.Range(), err)
	}

	defineTree, ok := root.Lookup(resolvedSrc)
	if !ok {
		err := root.LoadSubTree(resolvedSrc)
		if err != nil {
			return nil, errors.E(err, usesSchemasHCL.Source.Range, "source '%s' could not be loaded", src)
		}

		defineTree, ok = root.Lookup(resolvedSrc)
		if !ok {
			return nil, errors.E(usesSchemasHCL.Source.Range, "source '%s' not found", src)
		}
	}

	if len(defineTree.Node.Defines) == 0 {
		return nil, errors.E(usesSchemasHCL.Source.Range, "source '%s' contains no schema definitions", src)
	}

	var defineSchemas []*hcl.DefineSchema
	for _, define := range defineTree.Node.Defines {
		defineSchemas = append(defineSchemas, define.Schemas...)
	}

	if len(defineSchemas) == 0 {
		return nil, errors.E(usesSchemasHCL.Source.Range, "source '%s' contains no schema definitions", src)
	}

	var ret []*typeschema.Schema
	for _, schemaHCL := range defineSchemas {
		schema, err := EvalDefineSchema(evalctx, schemaHCL)
		if err != nil {
			return nil, err
		}
		ret = append(ret, schema)
	}

	return ret, nil
}

// EvalDefineSchema evaluates a schema definition into a Schema.
func EvalDefineSchema(evalctx *eval.Context, schemaHCL *hcl.DefineSchema) (*typeschema.Schema, error) {
	ret := &typeschema.Schema{
		Name: schemaHCL.Name,
	}

	typeStr := "any"

	configAttrs, err := EvalObjectAttributes(evalctx, schemaHCL.ObjectAttributes)
	if err != nil {
		return nil, err
	}
	if len(configAttrs) > 0 {
		typeStr = "object"
	}

	if schemaHCL.Type != nil {
		typeStr = getExprTokens(schemaHCL.Type.Expr)
	}

	ret.Type, err = typeschema.Parse(typeStr, extractSchemaAttrs(configAttrs))
	if err != nil {
		return nil, err
	}
	if err := typeschema.ValidateBundleRefPositions(ret.Type); err != nil {
		return nil, errors.E(err, schemaHCL.DefRange, "schema '%s'", schemaHCL.Name)
	}

	return ret, nil
}

// EvalObjectAttributes evaluates HCL object attribute definitions into config-level attributes.
func EvalObjectAttributes(evalctx *eval.Context, attrsHCL []*hcl.DefineObjectAttribute) ([]*ObjectAttribute, error) {
	var ret []*ObjectAttribute

	for _, attrHCL := range attrsHCL {
		var err error

		typeStr := "any"
		if attrHCL.Type != nil {
			typeStr = getExprTokens(attrHCL.Type.Expr)
		}
		attrTyp, err := typeschema.Parse(typeStr, nil)
		if err != nil {
			return nil, errors.E(err, "failed to parse typestr %s", typeStr)
		}
		if err := typeschema.ValidateBundleRefPositions(attrTyp); err != nil {
			return nil, errors.E(err, attrHCL.DefRange, "attribute '%s'", attrHCL.Name)
		}

		desc := ""
		if attrHCL.Description != nil {
			desc, err = EvalString(evalctx, attrHCL.Description.Expr, "description")
			if err != nil {
				return nil, err
			}
		}

		required := false
		if attrHCL.Required != nil {
			required, err = EvalBool(evalctx, attrHCL.Required.Expr, "required")
			if err != nil {
				return nil, err
			}
		}

		var prompt PromptConfig
		if p := attrHCL.Prompt; p != nil {
			if p.Text != nil {
				prompt.Text, err = EvalString(evalctx, p.Text.Expr, "prompt.text")
				if err != nil {
					return nil, err
				}
			}
			if p.Multiline != nil {
				prompt.Multiline, err = EvalBool(evalctx, p.Multiline.Expr, "prompt.multiline")
				if err != nil {
					return nil, err
				}
			}
			if p.Multiselect != nil {
				prompt.Multiselect, err = EvalBool(evalctx, p.Multiselect.Expr, "prompt.multiselect")
				if err != nil {
					return nil, err
				}
			}
			if p.Options != nil {
				prompt.optionsExpr = p.Options.Expr
			}
			if p.Condition != nil {
				prompt.conditionExpr = p.Condition.Expr
			}
		}

		ret = append(ret, &ObjectAttribute{
			Schema: &typeschema.ObjectTypeAttribute{
				Name:     attrHCL.Name,
				Type:     attrTyp,
				Required: required,
				Default:  attrHCL.Default,
			},
			Description: desc,
			Prompt:      prompt,
		})
	}

	return ret, nil
}

// extractSchemaAttrs extracts the type-level attributes from config-level ObjectAttributes.
func extractSchemaAttrs(attrs []*ObjectAttribute) []*typeschema.ObjectTypeAttribute {
	ret := make([]*typeschema.ObjectTypeAttribute, len(attrs))
	for i, a := range attrs {
		ret[i] = a.Schema
	}
	return ret
}

// EvalInputSchema evaluates an input definition into a type schema.
func EvalInputSchema(evalctx *eval.Context, inputHCL *hcl.DefineInput) (*typeschema.Schema, error) {
	schema := &typeschema.Schema{
		Name: inputHCL.Name,
	}

	typeStr := "any"
	configAttrs, err := EvalObjectAttributes(evalctx, inputHCL.ObjectAttributes)
	if err != nil {
		return nil, err
	}
	if len(configAttrs) > 0 {
		typeStr = "object"
	}

	if inputHCL.Type != nil {
		typeStr = getExprTokens(inputHCL.Type.Expr)
	}

	schema.Type, err = typeschema.Parse(typeStr, extractSchemaAttrs(configAttrs))
	if err != nil {
		return nil, errors.E(err, "failed to parse typestr %s", typeStr)
	}
	if err := typeschema.ValidateBundleRefPositions(schema.Type); err != nil {
		return nil, errors.E(err, inputHCL.DefRange, "input '%s'", inputHCL.Name)
	}

	return schema, nil
}

func getExprTokens(expr hhcl.Expression) string {
	tokens := ast.TokensForExpression(expr).Bytes()
	return strings.TrimSpace(string(tokens))
}

func applyInputSchema(name string, v cty.Value, sc typeschema.EvalContext) (cty.Value, error) {
	schema, err := sc.Schemas.Lookup("input." + name)
	if err != nil {
		return v, err
	}
	v, err = schema.Apply(v, sc)
	if err != nil {
		return v, err
	}
	return resolveBundleRefs(schema.Type, v, sc, name)
}

// resolveBundleRefs resolves every bundle-typed position in val by calling
// tm_bundle. Bundle references are not necessarily the whole input value: they
// can be nested inside collections (`list(bundle(...))`), object attributes or
// schema references, so the value is walked alongside its type.
func resolveBundleRefs(typ typeschema.Type, val cty.Value, sc typeschema.EvalContext, inputName string) (cty.Value, error) {
	return typeschema.MapBundleRefs(typ, val, sc,
		func(bt *typeschema.BundleType, ref cty.Value, path typeschema.BundleRefPath) (cty.Value, error) {
			return resolveBundleRef(bt, ref, sc.Evalctx, inputName, path)
		})
}

// resolveBundleRef resolves a single bundle reference by calling tm_bundle.
// Accepted reference values:
//   - null: passed through as-is
//   - string: a bundle key (alias or UUID)
//   - tuple/list [key, envID]: a bundle key with an explicit environment ID
//   - object/map: an already resolved bundle, passed through as-is
//
// An unresolvable reference yields a null value, which callers detect and report
// with a domain specific message.
func resolveBundleRef(bt *typeschema.BundleType, v cty.Value, evalctx *eval.Context, inputName string, path typeschema.BundleRefPath) (cty.Value, error) {
	if !v.IsKnown() || v.IsNull() {
		return v, nil
	}

	var args []cty.Value

	switch {
	case v.Type() == cty.String:
		args = []cty.Value{cty.StringVal(bt.ClassID), v}

	case v.Type().IsTupleType() || v.Type().IsListType():
		elems := v.AsValueSlice()
		if len(elems) != 2 {
			return v, errors.E("bundle input %q expects a [key, envID] tuple of length 2, got %d",
				inputName+path.String(), len(elems))
		}
		args = []cty.Value{cty.StringVal(bt.ClassID), elems[0], elems[1]}

	case v.Type().IsObjectType() || v.Type().IsMapType():
		return v, nil

	default:
		return v, errors.E("unexpected value type %s for bundle input %q",
			v.Type().FriendlyName(), inputName+path.String())
	}

	hclctx := evalctx.Unwrap()
	fn, ok := hclctx.Functions["tm_bundle"]
	if !ok {
		return v, errors.E("tm_bundle function not available in eval context")
	}

	result, err := fn.Call(args)
	if err != nil {
		return v, errors.E(err, "resolving bundle input %q", inputName+path.String())
	}
	return result, nil
}
