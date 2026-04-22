// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/preempt"
)

// Registry stores several lists of common objects we may want to look up.
type Registry struct {
	Environments []*Environment
	Bundles      []*Bundle
}

// BundleAliasAwaitKey computes the preempt await key for a bundle alias and environment ID.
func BundleAliasAwaitKey(alias, envID string) string {
	return fmt.Sprintf("%s:%s", envID, alias)
}

// BundleUUIDAwaitKey computes the preempt await key for a bundle UUID and environment ID.
func BundleUUIDAwaitKey(uuid, envID string) string {
	return fmt.Sprintf("%s:%s", envID, uuid)
}

// BundleAwaitKeys returns all preempt await keys for the given bundle.
func BundleAwaitKeys(b *Bundle) []string {
	envID := ""
	if b.Environment != nil {
		envID = b.Environment.ID
	}
	keys := []string{BundleAliasAwaitKey(b.Alias, envID)}
	if b.UUID != "" {
		keys = append(keys, BundleUUIDAwaitKey(b.UUID, envID))
	}
	return keys
}

// BundleFunc returns the `tm_bundle` function.
func BundleFunc(ctx context.Context, reg *Registry, currentEnv *Environment, useAwait bool) function.Function {
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

			var keyKind string
			var pred func(*Bundle) bool
			var awaitKey string

			if err := uuid.Validate(key); err == nil {
				keyKind = "UUID"
				pred = func(b *Bundle) bool {
					return b.UUID == key
				}
				awaitKey = BundleUUIDAwaitKey(key, envID)

			} else {
				keyKind = "alias"
				pred = func(b *Bundle) bool {
					return b.Alias == key
				}
				awaitKey = BundleAliasAwaitKey(key, envID)
			}

			if useAwait {
				// This waits until the given preemptKey is ready.
				if err := preempt.Await(ctx, awaitKey); err != nil {
					if errors.IsKind(err, preempt.ErrUnresolvable) {
						return cty.NilVal, errors.E("bundle with %s %q could not be resolved - either missing, or circular dependency", keyKind, key)
					}
					return cty.NilVal, err
				}
			}

			for _, b := range reg.Bundles {
				if b.DefinitionMetadata.Class != class || !pred(b) {
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
func BundlesFunc(reg *Registry, currentEnv *Environment) function.Function {
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
			if len(reg.Bundles) == 0 {
				return cty.EmptyTupleVal, nil
			}

			class := args[0].AsString()

			envID := ""
			if len(args) > 1 {
				envID = args[1].AsString()
			} else if currentEnv != nil {
				envID = currentEnv.ID
			}

			var matched []*Bundle
			for _, b := range reg.Bundles {
				if class != "*" && class != b.DefinitionMetadata.Class {
					continue
				}
				// If the bundle has an environment, match it against the current environment.
				// If not (bundles that don't required environments), ignore it completely.
				if b.Environment != nil {
					if envID != b.Environment.ID {
						continue
					}
				}
				matched = append(matched, b)
			}
			if len(matched) == 0 {
				return cty.EmptyTupleVal, nil
			}

			slices.SortFunc(matched, func(a, b *Bundle) int {
				aEnv, bEnv := "", ""
				if a.Environment != nil {
					aEnv = a.Environment.ID
				}
				if b.Environment != nil {
					bEnv = b.Environment.ID
				}
				return cmp.Or(
					cmp.Compare(a.DefinitionMetadata.Class, b.DefinitionMetadata.Class),
					cmp.Compare(aEnv, bEnv),
					cmp.Compare(a.Alias, b.Alias),
				)
			})

			r := make([]cty.Value, 0, len(matched))
			for _, b := range matched {
				r = append(r, MakeObjectFromBundle(b))
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
		"class":       cty.StringVal(b.DefinitionMetadata.Class),
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
