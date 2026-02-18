// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:generate goyacc -o parser.go parser.y

// Package typeschema provides type definitions and validation for Terramate schemas.
package typeschema

import (
	"fmt"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

// Parse parses a type string and optional inline attributes into a Type.
func Parse(typeStr string, inlineAttrs []*ObjectTypeAttribute) (Type, error) {
	l := NewLexer(typeStr)
	if yyParse(l) != 0 {
		return nil, l.Err
	}

	if len(inlineAttrs) > 0 {
		matched := false

		// Attach inlineAttrs to "object"s.
		err := l.Result.Visit(func(v Type) error {
			switch v := v.(type) {
			case *ObjectType:
				v.Attributes = inlineAttrs
				matched = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}

		if !matched {
			return nil, errors.E("defining attributes requires use of 'object' in type, got '%s'", typeStr)
		}
	}

	return l.Result, nil
}

// Type is the interface for all type representations.
type Type interface {
	Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, strict bool) (cty.Value, error)

	Visit(func(Type) error) error

	String() string
}

// PrimitiveType represents a primitive type (string, bool, number).
type PrimitiveType struct {
	Name string
}

// Apply validates and coerces the given value to match this primitive type.
func (p PrimitiveType) Apply(val cty.Value, _ *eval.Context, _ SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	switch p.Name {
	case "string":
		if val.Type() != cty.String {
			return val, errors.E("expected string, got %s", val.Type().FriendlyName())
		}
	case "bool":
		if val.Type() != cty.Bool {
			return val, errors.E("expected bool, got %s", val.Type().FriendlyName())
		}
	case "number":
		if val.Type() != cty.Number {
			return val, errors.E("expected number, got %s", val.Type().FriendlyName())
		}
	}
	return val, nil
}

// Visit calls fn on this type.
func (p *PrimitiveType) Visit(fn func(Type) error) error {
	return fn(p)
}

func (p PrimitiveType) String() string {
	return p.Name
}

// Collection Types

// ListType represents a list type with a value type constraint.
type ListType struct {
	ValueType Type
}

// Apply validates and coerces the given value to match this list type.
func (l ListType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	if !val.Type().IsListType() && !val.Type().IsTupleType() {
		return val, errors.E("expected list, got %s", val.Type().FriendlyName())
	}
	if val.LengthInt() == 0 {
		return val, nil
	}

	vals := val.AsValueSlice()

	for i, elemVal := range vals {
		var err error
		vals[i], err = l.ValueType.Apply(elemVal, evalctx, schemas, true)
		if err != nil {
			return val, errors.E("list element error: %w", err)
		}
	}

	return cty.TupleVal(vals), nil
}

// Visit calls fn on this type and its value type.
func (l *ListType) Visit(fn func(Type) error) error {
	if err := fn(l); err != nil {
		return err
	}
	return l.ValueType.Visit(fn)
}

func (l ListType) String() string {
	return fmt.Sprintf("list(%s)", l.ValueType.String())
}

// SetType represents a set type with a value type constraint.
type SetType struct {
	ValueType Type
}

// Apply validates and coerces the given value to match this set type.
func (s SetType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	if !val.Type().IsListType() && !val.Type().IsTupleType() && !val.Type().IsSetType() {
		return val, errors.E("expected set or list, got %s", val.Type().FriendlyName())
	}
	if val.LengthInt() == 0 {
		return val, nil
	}

	vals := val.AsValueSlice()

	// Remove duplicates while preserving order and applying type constraints
	seen := make(map[string]bool)
	uniqueVals := make([]cty.Value, 0, len(vals))

	for _, elemVal := range vals {
		appliedVal, err := s.ValueType.Apply(elemVal, evalctx, schemas, true)
		if err != nil {
			return val, errors.E("set element error: %w", err)
		}

		// Use GoString as a key for deduplication (consistent with HCL behavior)
		key := appliedVal.GoString()
		if !seen[key] {
			seen[key] = true
			uniqueVals = append(uniqueVals, appliedVal)
		}
	}

	return cty.TupleVal(uniqueVals), nil
}

// Visit calls fn on this type and its value type.
func (s *SetType) Visit(fn func(Type) error) error {
	if err := fn(s); err != nil {
		return err
	}
	return s.ValueType.Visit(fn)
}

func (s SetType) String() string {
	return fmt.Sprintf("set(%s)", s.ValueType.String())
}

// MapType represents a map type with a value type constraint.
type MapType struct {
	ValueType Type
}

// Apply validates and coerces the given value to match this map type.
func (m MapType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	if !val.Type().IsMapType() && !val.Type().IsObjectType() {
		return val, errors.E("expected map, got %s", val.Type().FriendlyName())
	}
	if val.LengthInt() == 0 {
		return val, nil
	}

	valMap := val.AsValueMap()
	if valMap == nil {
		valMap = map[string]cty.Value{}
	}

	for k, elemVal := range valMap {
		var err error
		valMap[k], err = m.ValueType.Apply(elemVal, evalctx, schemas, true)
		if err != nil {
			return val, errors.E("map value error: %w", err)
		}
	}

	// We have to use ObjectVal here instead of MapVal, just like we use TupleVal for lists,
	// because MapVal will panic if elements are not of the same type.
	return cty.ObjectVal(valMap), nil
}

// Visit calls fn on this type and its value type.
func (m MapType) Visit(fn func(Type) error) error {
	if err := fn(m); err != nil {
		return err
	}
	return m.ValueType.Visit(fn)
}

func (m MapType) String() string {
	return fmt.Sprintf("map(%s)", m.ValueType.String())
}

// TupleType represents a tuple type with element type constraints.
type TupleType struct {
	Elems []Type
}

// Apply validates and coerces the given value to match this tuple type.
func (t TupleType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return val, errors.E("expected tuple, got %s", val.Type().FriendlyName())
	}
	if val.LengthInt() != len(t.Elems) {
		return val, errors.E("expected tuple of length %d, got %d", len(t.Elems), val.LengthInt())
	}
	if val.LengthInt() == 0 {
		return val, nil
	}

	vals := val.AsValueSlice()
	for i, elemVal := range vals {
		var err error
		vals[i], err = t.Elems[i].Apply(elemVal, evalctx, schemas, true)
		if err != nil {
			return val, errors.E("tuple item %d: %w", i, err)
		}
	}
	return cty.TupleVal(vals), nil
}

// Visit calls fn on this type and all element types.
func (t *TupleType) Visit(fn func(Type) error) error {
	if err := fn(t); err != nil {
		return err
	}
	for _, e := range t.Elems {
		if err := e.Visit(fn); err != nil {
			return err
		}
	}
	return nil
}

func (t TupleType) String() string {
	strs := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		strs[i] = e.String()
	}
	return fmt.Sprintf("tuple(%s)", strings.Join(strs, ", "))
}

// ReferenceType represents a reference to a named schema type.
type ReferenceType struct {
	Name string
}

// Apply resolves the reference and delegates to the referenced schema type.
func (r ReferenceType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, strict bool) (cty.Value, error) {
	schema, err := schemas.Lookup(r.Name)
	if err != nil {
		return val, err
	}

	// Forward strict
	return schema.Type.Apply(val, evalctx, schemas, strict)
}

// Visit calls fn on this type.
func (r *ReferenceType) Visit(fn func(Type) error) error {
	return fn(r)
}

func (r ReferenceType) String() string {
	return r.Name
}

// NonStrictType wraps a type to disable strict validation.
type NonStrictType struct {
	Inner Type
}

// Apply delegates to the inner type with strict mode disabled.
func (n NonStrictType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	return n.Inner.Apply(val, evalctx, schemas, false)
}

// Visit calls fn on this type and its inner type.
func (n *NonStrictType) Visit(fn func(Type) error) error {
	if err := fn(n); err != nil {
		return err
	}
	return n.Inner.Visit(fn)
}

func (n NonStrictType) String() string {
	// Always show brackets
	return "has(" + n.Inner.String() + ")"
}

// ObjectType represents a structured object type with named attributes.
type ObjectType struct {
	Attributes []*ObjectTypeAttribute
}

// ObjectTypeAttribute defines a single attribute within an object type.
type ObjectTypeAttribute struct {
	Name        string
	Type        Type
	Description string
	Default     *ast.Attribute
	Required    bool
	Range       info.Range
}

// Apply validates and coerces the given value to match this object type.
func (o ObjectType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, strict bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	if !val.Type().IsObjectType() && !val.Type().IsMapType() {
		return val, errors.E("expected object-like type")
	}

	valMap := val.AsValueMap()
	if valMap == nil {
		valMap = map[string]cty.Value{}
	}

	for _, attr := range o.Attributes {
		fieldVal, exists := valMap[attr.Name]

		if !exists {
			if attr.Default != nil {
				var err error
				fieldVal, err = evalctx.Eval(attr.Default.Expr)
				if err != nil {
					return val, err
				}
				valMap[attr.Name] = fieldVal
			} else if attr.Required {
				return val, errors.E("missing required attribute '%s'", attr.Name)
			} else {
				// Optional attribute with no default - skip it
				continue
			}
		}

		var err error
		fieldVal, err = attr.Type.Apply(fieldVal, evalctx, schemas, strict)
		if err != nil {
			return val, errors.E(err, "failed to Apply '%s'", attr.Name)
		}
		valMap[attr.Name] = fieldVal
	}

	if strict && len(o.Attributes) > 0 {
		for name := range valMap {
			found := false
			for _, attr := range o.Attributes {
				if attr.Name == name {
					found = true
					break
				}
			}
			if !found {
				return val, errors.E("unexpected attribute '%s'", name)
			}
		}
	}

	return cty.ObjectVal(valMap), nil
}

// Visit calls fn on this type and all attribute types.
func (o *ObjectType) Visit(fn func(Type) error) error {
	if err := fn(o); err != nil {
		return err
	}
	for _, attr := range o.Attributes {
		if err := attr.Type.Visit(fn); err != nil {
			return err
		}
	}
	return nil
}

func (o ObjectType) String() string {
	return "object"
}

// VariantType represents a union type that matches any of its options.
type VariantType struct {
	Options []Type
}

// Apply tries each variant option and returns the first successful match.
func (v VariantType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}
	var errs []string
	for _, opt := range v.Options {
		newVal, err := opt.Apply(val, evalctx, schemas, true)
		if err == nil {
			return newVal, nil
		}
		errs = append(errs, err.Error())
	}
	return val, errors.E("no variant matched: [%s]", strings.Join(errs, " | "))
}

// Visit calls fn on this type and all option types.
func (v *VariantType) Visit(fn func(Type) error) error {
	if err := fn(v); err != nil {
		return err
	}
	for _, o := range v.Options {
		if err := o.Visit(fn); err != nil {
			return err
		}
	}
	return nil
}

func (v VariantType) String() string {
	strs := make([]string, len(v.Options))
	for i, o := range v.Options {
		strs[i] = o.String()
	}
	return "any_of(" + strings.Join(strs, ", ") + ")"
}

// MergedObjectType represents a type formed by merging multiple object types.
type MergedObjectType struct {
	Objects []Type
}

// Apply validates the value against all merged object attributes.
func (m MergedObjectType) Apply(val cty.Value, evalctx *eval.Context, schemas SchemaNamespaces, strict bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}

	var mergedAttrs []*ObjectTypeAttribute
	duplicateCheckSet := map[string]*ObjectTypeAttribute{}

	for _, t := range m.Objects {
		var obj *ObjectType

		switch x := t.(type) {
		case *ReferenceType:
			schema, err := schemas.Lookup(x.Name)
			if err != nil {
				return val, err
			}
			var ok bool
			obj, ok = schema.Type.(*ObjectType)
			if !ok {
				return val, errors.E("cannot merge type '%s': expected ObjectType, got %T", x.Name, schema.Type)
			}
		case *ObjectType:
			obj = x
		default:
			return val, errors.E("cannot merge type %T: expected ReferenceType or ObjectType", t)
		}

		for _, attr := range obj.Attributes {
			if dup, found := duplicateCheckSet[attr.Name]; found {
				return val, errors.E(
					"cannot merge types '%s': attribute %s is defined twice. first at %s, second definition at %s.",
					m.String(),
					attr.Name,
					dup.Range.String(),
					attr.Range.String(),
				)
			}
			duplicateCheckSet[attr.Name] = attr
			mergedAttrs = append(mergedAttrs, attr)
		}
	}

	// Forward strict
	tempSchema := &ObjectType{Attributes: mergedAttrs}
	return tempSchema.Apply(val, evalctx, schemas, strict)
}

// Visit calls fn on this type and all constituent object types.
func (m *MergedObjectType) Visit(fn func(Type) error) error {
	if err := fn(m); err != nil {
		return err
	}
	for _, obj := range m.Objects {
		if err := obj.Visit(fn); err != nil {
			return err
		}
	}
	return nil
}

func (m MergedObjectType) String() string {
	parts := make([]string, len(m.Objects))
	for i, obj := range m.Objects {
		parts[i] = obj.String()
	}
	return strings.Join(parts, " + ")
}

// AnyType matches anything.
type AnyType struct{}

// Apply returns the value unchanged since AnyType matches anything.
func (a AnyType) Apply(val cty.Value, _ *eval.Context, _ SchemaNamespaces, _ bool) (cty.Value, error) {
	return val, nil
}

// Visit calls fn on this type.
func (a *AnyType) Visit(fn func(Type) error) error {
	return fn(a)
}

func (a AnyType) String() string {
	return "any"
}

// UnwrapValueType returns the inner value type for collection types.
func UnwrapValueType(typ Type) Type {
	switch x := typ.(type) {
	case *ListType:
		return x.ValueType
	case *SetType:
		return x.ValueType
	case *MapType:
		return x.ValueType
	}
	return typ
}

// IsCollectionType returns true if the type is a list, set, or map.
func IsCollectionType(typ Type) bool {
	switch typ.(type) {
	case *ListType, *SetType, *MapType:
		return true
	}
	return false
}

// BundleType represents a bundle reference type.
type BundleType struct {
	ClassID string
}

// Apply validates the given value for a bundle type input.
// Accepted value types:
//   - null: passed through as-is
//   - string: a bundle key (alias or UUID)
//   - tuple/list [key, envID]: a bundle key with an explicit environment ID
//
// This method only validates; it does NOT resolve the bundle.
func (b BundleType) Apply(val cty.Value, _ *eval.Context, _ SchemaNamespaces, _ bool) (cty.Value, error) {
	if val.IsNull() {
		return val, nil
	}

	switch {
	case val.Type() == cty.String:
		return val, nil

	case val.Type().IsTupleType() || val.Type().IsListType():
		elems := val.AsValueSlice()
		if len(elems) != 2 {
			return val, errors.E("bundle type expects a [key, envID] tuple of length 2, got %d", len(elems))
		}
		for i, e := range elems {
			if e.Type() != cty.String {
				return val, errors.E("bundle type tuple element %d must be string, got %s", i, e.Type().FriendlyName())
			}
		}
		return val, nil

	default:
		return val, errors.E("bundle type expects a string or [key, envID] tuple, got %s", val.Type().FriendlyName())
	}
}

// Visit calls fn on this type.
func (b *BundleType) Visit(fn func(Type) error) error {
	return fn(b)
}

func (b BundleType) String() string {
	return fmt.Sprintf("bundle(%q)", b.ClassID)
}

// IsAnyType returns true if the type is AnyType.
func IsAnyType(typ Type) bool {
	switch typ.(type) {
	case *AnyType:
		return true
	}
	return false
}
