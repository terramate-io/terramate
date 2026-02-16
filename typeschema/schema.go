// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package typeschema

import (
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

// Schema represents a named type schema with metadata.
type Schema struct {
	Name        string
	Description string
	Range       info.Range
	Type        Type
}

// Apply validates and coerces the given value against this schema's type.
func (s *Schema) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces) (cty.Value, error) {
	return s.Type.Apply(val, evalctx, schemas, true)
}

// SchemaNamespaces holds schemas organized by namespace.
type SchemaNamespaces struct {
	items map[string]map[string]*Schema
}

// NewSchemaNamespaces creates an empty SchemaNamespaces.
func NewSchemaNamespaces() SchemaNamespaces {
	return SchemaNamespaces{items: make(map[string]map[string]*Schema)}
}

// Set registers a list of schemas under the given namespace.
func (s *SchemaNamespaces) Set(namespace string, schemas []*Schema) {
	t := make(map[string]*Schema, len(schemas))
	for _, s := range schemas {
		t[s.Name] = s
	}
	s.items[namespace] = t
}

// Lookup resolves a schema by its fully-qualified ID (namespace.name).
func (s SchemaNamespaces) Lookup(schemaID string) (*Schema, error) {
	namespace, name, found := strings.Cut(schemaID, ".")
	if !found || namespace == "" || name == "" {
		return nil, errors.E("invalid schema reference '%s', expected format '<namespace>.<name>'", schemaID)
	}
	schemas, found := s.items[namespace]
	if !found {
		return nil, errors.E("schema namespace '%s' does not exist", namespace)
	}
	schema, found := schemas[name]
	if !found {
		return nil, errors.E("schema '%s' does not exist in namespace '%s'", name, namespace)
	}
	return schema, nil
}
