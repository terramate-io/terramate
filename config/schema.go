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

	inlineAttrs, err := EvalObjectAttributes(evalctx, schemaHCL.ObjectAttributes)
	if err != nil {
		return nil, err
	}
	if len(inlineAttrs) > 0 {
		typeStr = "object"
	}

	if schemaHCL.Type != nil {
		typeStr = getExprTokens(schemaHCL.Type.Expr)
	}

	ret.Type, err = typeschema.Parse(typeStr, inlineAttrs)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// EvalObjectAttributes evaluates HCL object attribute definitions into type attributes.
func EvalObjectAttributes(evalctx *eval.Context, attrsHCL []*hcl.DefineObjectAttribute) ([]*typeschema.ObjectTypeAttribute, error) {
	var ret []*typeschema.ObjectTypeAttribute

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

		ret = append(ret, &typeschema.ObjectTypeAttribute{
			Name:        attrHCL.Name,
			Type:        attrTyp,
			Description: desc,
			Required:    required,
			// The default is not evaluated yet.
			// It may reference bundle inputs, so it must be evaluated later during bundle input evaluation.
			Default: attrHCL.Default,
			//Range:       attrHCL.DefRange, // TODO
		})
	}

	return ret, nil
}

// EvalInputSchema evaluates an input definition into a type schema.
func EvalInputSchema(evalctx *eval.Context, inputHCL *hcl.DefineInput) (*typeschema.Schema, error) {
	schema := &typeschema.Schema{
		Name: inputHCL.Name,
	}

	typeStr := "any"

	inlineAttrs, err := EvalObjectAttributes(evalctx, inputHCL.ObjectAttributes)
	if err != nil {
		return nil, err
	}
	if len(inlineAttrs) > 0 {
		typeStr = "object"
	}

	if inputHCL.Type != nil {
		typeStr = getExprTokens(inputHCL.Type.Expr)
	}

	schema.Type, err = typeschema.Parse(typeStr, inlineAttrs)
	if err != nil {
		return nil, errors.E(err, "failed to parse typestr %s", typeStr)
	}

	return schema, nil
}

func getExprTokens(expr hhcl.Expression) string {
	tokens := ast.TokensForExpression(expr).Bytes()
	return strings.TrimSpace(string(tokens))
}

func applyInputSchema(name string, v cty.Value, evalctx *eval.Context, schemas typeschema.SchemaNamespaces) (cty.Value, error) {
	schema, err := schemas.Lookup("input." + name)
	if err != nil {
		return v, err
	}
	return schema.Apply(v, evalctx, schemas)
}
