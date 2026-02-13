// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
)

// BundleFunc returns the `tm_bundle` function.
func BundleFunc(bundles []*Bundle, currentEnv *Environment) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "class",
				Type: cty.String,
			},
			{
				Name: "key",
				Type: cty.String,
			},
		},
		VarParam: &function.Parameter{
			Name: "env_id",
			Type: cty.String,
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			class := args[0].AsString()
			key := args[1].AsString()

			envID := ""
			if len(args) > 2 {
				envID = args[2].AsString()
			} else if currentEnv != nil {
				envID = currentEnv.ID
			}

			var pred func(*Bundle) bool

			if err := uuid.Validate(key); err == nil {
				pred = func(b *Bundle) bool {
					return b.UUID == key
				}
			} else {
				pred = func(b *Bundle) bool {
					return b.Alias == key
				}
			}

			for _, b := range bundles {
				if b.Class != class || !pred(b) {
					continue
				}
				// If the bundle has an environment, match it against the current environment.
				// If not (bundles that don't required environments), ignore it completely.
				if b.Environment != nil {
					if envID != b.Environment.ID {
						continue
					}
				}

				return MakeObjectFromBundle(b), nil

			}

			return cty.NullVal(cty.DynamicPseudoType), nil
		},
	})
}

// BundlesFunc returns the `tm_bundles` function.
func BundlesFunc(bundles []*Bundle, currentEnv *Environment) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "class",
				Type: cty.String,
			},
		},
		VarParam: &function.Parameter{
			Name: "env",
			Type: cty.String,
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			if len(bundles) == 0 {
				return cty.EmptyTupleVal, nil
			}

			class := args[0].AsString()

			envID := ""
			if len(args) > 1 {
				envID = args[1].AsString()
			} else if currentEnv != nil {
				envID = currentEnv.ID
			}

			var r []cty.Value
			for _, b := range bundles {
				if class != "*" && class != b.Class {
					continue
				}
				// If the bundle has an environment, match it against the current environment.
				// If not (bundles that don't required environments), ignore it completely.
				if b.Environment != nil {
					if envID != b.Environment.ID {
						continue
					}
				}

				r = append(r, MakeObjectFromBundle(b))
			}

			if len(r) == 0 {
				return cty.EmptyTupleVal, nil
			}
			return cty.TupleVal(r), nil
		},
	})
}

// MakeObjectFromBundle converts a Bundle into a cty object value.
func MakeObjectFromBundle(b *Bundle) cty.Value {
	var uuidVal cty.Value
	if b.UUID != "" {
		uuidVal = cty.StringVal(b.UUID)
	} else {
		uuidVal = cty.NullVal(cty.String)
	}

	return cty.ObjectVal(map[string]cty.Value{
		"alias":       cty.StringVal(b.Alias),
		"class":       cty.StringVal(b.Class),
		"uuid":        uuidVal,
		"input":       cty.ObjectVal(b.Inputs),
		"export":      cty.ObjectVal(b.Exports),
		"environment": MakeEnvObject(b.Environment),
	})
}

// TmSourceFunc returns the `tm_source` function.
func TmSourceFunc(stackDir, compSrc, bundleSrc string) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "path",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
			p := args[0].AsString()

			if len(p) == 0 {
				return cty.NullVal(cty.String), errors.E("path must not be empty")
			}

			if compSrc != "" {
				p = resolve.CombineSources(p, compSrc)
			}
			if bundleSrc != "" {
				p = resolve.CombineSources(p, bundleSrc)
			}

			// If url, use it.
			if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, ".") {
				return cty.StringVal(p), nil
			}

			// Otherwise, adjust relative to stack dir.
			r, err := filepath.Rel(stackDir, p)
			if err != nil {
				return cty.NullVal(cty.String), err
			}

			return cty.StringVal(filepath.ToSlash(r)), nil
		},
	})
}
