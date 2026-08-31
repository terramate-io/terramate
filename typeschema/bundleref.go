// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package typeschema

import (
	"strconv"
	"strings"

	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/errors"
)

// BundleRefPath locates a bundle reference inside a structured input value.
// The zero value refers to the value itself.
type BundleRefPath []string

// Index returns the path of the i-th element of the collection at p.
func (p BundleRefPath) Index(i int) BundleRefPath {
	return p.child("[" + strconv.Itoa(i) + "]")
}

// Key returns the path of the map entry k at p.
func (p BundleRefPath) Key(k string) BundleRefPath {
	return p.child("[" + strconv.Quote(k) + "]")
}

// AnyKey returns the path of any entry of the map at p, for messages that talk
// about the map's value type rather than one entry.
func (p BundleRefPath) AnyKey() BundleRefPath {
	return p.child("[*]")
}

// Attr returns the path of the object attribute name at p.
func (p BundleRefPath) Attr(name string) BundleRefPath {
	return p.child("." + name)
}

func (p BundleRefPath) child(step string) BundleRefPath {
	// Copy so that sibling paths never share the backing array.
	out := make(BundleRefPath, len(p), len(p)+1)
	copy(out, p)
	return append(out, step)
}

// IsRoot reports whether p points at the value itself.
func (p BundleRefPath) IsRoot() bool { return len(p) == 0 }

// String renders the path as a suffix that can be appended to an input name,
// e.g. `[0].team`. It is empty for the root.
func (p BundleRefPath) String() string { return strings.Join(p, "") }

// BundleRefFunc is called by [MapBundleRefs] for every bundle-typed position in
// a value. It returns the replacement value for that position.
type BundleRefFunc func(bt *BundleType, val cty.Value, path BundleRefPath) (cty.Value, error)

// MapBundleRefs walks val alongside typ and calls fn at every position typed
// `bundle(<class>)`, returning val with fn's results substituted in.
//
// Bundle references can be nested inside collections (`list(bundle(...))`),
// object attributes and schema references, so everything that translates
// between the two representations of a reference — the key that is written to
// disk and the resolved bundle object — has to walk the type tree instead of
// only looking at the top level.
//
// Collections are rebuilt as tuples and maps as objects, mirroring
// [Type.Apply], because resolved bundles of the same class still have different
// cty types: their input and export attributes differ per bundle.
//
// Subtrees that hold no bundle reference are returned untouched, and so are
// values whose shape does not match typ. [Type.Apply] is responsible for
// validation; this is a value transformation only.
func MapBundleRefs(typ Type, val cty.Value, schemactx EvalContext, fn BundleRefFunc) (cty.Value, error) {
	v, _, err := mapBundleRefs(typ, val, schemactx, nil, fn)
	return v, err
}

// mapBundleRefs reports whether it replaced anything, so that untouched
// subtrees keep their original value instead of being rebuilt.
func mapBundleRefs(typ Type, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	if val == cty.NilVal {
		return val, false, nil
	}

	switch t := typ.(type) {
	case *BundleType:
		out, err := fn(t, val, path)
		if err != nil {
			return val, false, err
		}
		return out, true, nil

	case *ListType:
		return mapBundleRefsInSeq(t.ValueType, val, schemactx, path, fn)

	case *SetType:
		return mapBundleRefsInSeq(t.ValueType, val, schemactx, path, fn)

	case *TupleType:
		return mapBundleRefsInTuple(t, val, schemactx, path, fn)

	case *MapType:
		return mapBundleRefsInMap(t.ValueType, val, schemactx, path, fn)

	case *ObjectType:
		return mapBundleRefsInAttrs(t.Attributes, val, schemactx, path, fn)

	case *MergedObjectType:
		return mapBundleRefsInAttrs(mergedAttributes(t, schemactx), val, schemactx, path, fn)

	case *ReferenceType:
		schema, err := schemactx.Schemas.Lookup(t.Name)
		if err != nil {
			// Apply already validated the reference. Without the schema there
			// is nothing to map, which is not an error for a transformation.
			return val, false, nil
		}
		return mapBundleRefs(schema.Type, val, schemactx, path, fn)

	case *NonStrictType:
		return mapBundleRefs(t.Inner, val, schemactx, path, fn)

	case *VariantType:
		return mapBundleRefsInVariant(t, val, schemactx, path, fn)
	}

	return val, false, nil
}

func mapBundleRefsInSeq(elemType Type, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	if !isMappableCollection(val) || !isSequenceLike(val) {
		return val, false, nil
	}
	elems := val.AsValueSlice()
	if len(elems) == 0 {
		return val, false, nil
	}

	out := make([]cty.Value, len(elems))
	changed := false
	for i, elem := range elems {
		mapped, elemChanged, err := mapBundleRefs(elemType, elem, schemactx, path.Index(i), fn)
		if err != nil {
			return val, false, err
		}
		out[i] = mapped
		changed = changed || elemChanged
	}
	if !changed {
		return val, false, nil
	}
	return cty.TupleVal(out), true, nil
}

func mapBundleRefsInTuple(t *TupleType, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	if !isMappableCollection(val) || !isSequenceLike(val) {
		return val, false, nil
	}
	elems := val.AsValueSlice()
	if len(elems) != len(t.Elems) {
		return val, false, nil
	}

	out := make([]cty.Value, len(elems))
	changed := false
	for i, elem := range elems {
		mapped, elemChanged, err := mapBundleRefs(t.Elems[i], elem, schemactx, path.Index(i), fn)
		if err != nil {
			return val, false, err
		}
		out[i] = mapped
		changed = changed || elemChanged
	}
	if !changed {
		return val, false, nil
	}
	return cty.TupleVal(out), true, nil
}

func mapBundleRefsInMap(valType Type, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	if !isMappableCollection(val) || !isObjectLike(val) {
		return val, false, nil
	}
	entries := val.AsValueMap()
	if len(entries) == 0 {
		return val, false, nil
	}

	changed := false
	for k, elem := range entries {
		mapped, elemChanged, err := mapBundleRefs(valType, elem, schemactx, path.Key(k), fn)
		if err != nil {
			return val, false, err
		}
		if elemChanged {
			entries[k] = mapped
			changed = true
		}
	}
	if !changed {
		return val, false, nil
	}
	return cty.ObjectVal(entries), true, nil
}

func mapBundleRefsInAttrs(attrs []*ObjectTypeAttribute, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	if len(attrs) == 0 || !isMappableCollection(val) || !isObjectLike(val) {
		return val, false, nil
	}

	// Start from the value's own attributes so that entries the type does not
	// cover - non-strict objects - survive the rebuild.
	out := val.AsValueMap()
	changed := false
	for _, attr := range attrs {
		elem, exists := out[attr.Name]
		if !exists {
			continue
		}
		mapped, attrChanged, err := mapBundleRefs(attr.Type, elem, schemactx, path.Attr(attr.Name), fn)
		if err != nil {
			return val, false, err
		}
		if attrChanged {
			out[attr.Name] = mapped
			changed = true
		}
	}
	if !changed {
		return val, false, nil
	}
	return cty.ObjectVal(out), true, nil
}

// mapBundleRefsInVariant maps the first option that accepts val, mirroring how
// [VariantType.Apply] picks the matching option.
func mapBundleRefsInVariant(t *VariantType, val cty.Value, schemactx EvalContext, path BundleRefPath, fn BundleRefFunc) (cty.Value, bool, error) {
	for _, opt := range t.Options {
		if _, err := opt.Apply(val, schemactx, true); err != nil {
			continue
		}
		return mapBundleRefs(opt, val, schemactx, path, fn)
	}
	return val, false, nil
}

// mergedAttributes collects the attributes of a merged object type. Duplicates
// and non-object operands are already rejected by [MergedObjectType.Apply].
func mergedAttributes(t *MergedObjectType, schemactx EvalContext) []*ObjectTypeAttribute {
	var attrs []*ObjectTypeAttribute
	for _, obj := range t.Objects {
		switch x := obj.(type) {
		case *ObjectType:
			attrs = append(attrs, x.Attributes...)
		case *ReferenceType:
			schema, err := schemactx.Schemas.Lookup(x.Name)
			if err != nil {
				continue
			}
			if objType, ok := schema.Type.(*ObjectType); ok {
				attrs = append(attrs, objType.Attributes...)
			}
		}
	}
	return attrs
}

func isMappableCollection(val cty.Value) bool {
	return val.IsKnown() && !val.IsNull() && val.CanIterateElements()
}

func isObjectLike(val cty.Value) bool {
	t := val.Type()
	return t.IsObjectType() || t.IsMapType()
}

func isSequenceLike(val cty.Value) bool {
	t := val.Type()
	return t.IsListType() || t.IsTupleType() || t.IsSetType()
}

// ValidateBundleRefPositions reports an error if a bundle reference appears in a
// position the CLI cannot edit. Supported are a reference on its own and a list
// or set of references:
//
//	bundle(<class>)
//	list(bundle(<class>))
//	set(bundle(<class>))
//
// Deeper nesting - map values, tuple elements, lists of lists, any_of options -
// would resolve correctly but has no editor that makes sense, so it is rejected
// at definition time rather than surfacing as a confusing form. Object
// attributes are types in their own right, so an attribute may hold any of the
// supported shapes; `list(object)` with a bundle-typed attribute is fine.
//
// Schema references are not followed: a referenced schema is validated where it
// is defined.
func ValidateBundleRefPositions(typ Type) error {
	path, found := findUnsupportedBundleRef(typ, nil, true, true)
	if !found {
		return nil
	}
	const rule = "a bundle reference must be the whole type, or the elements of a list or set"
	if path.IsRoot() {
		return errors.E("type %q is not supported: %s", typ.String(), rule)
	}
	return errors.E("type %q is not supported: the bundle reference at %s is not allowed, %s",
		typ.String(), path.String(), rule)
}

// findUnsupportedBundleRef returns the path of the first bundle reference in an
// unsupported position. allowed reports whether a reference may sit at this
// exact position; atRoot reports whether this position is the root of a type,
// which is the only place a list or set may still carry references.
func findUnsupportedBundleRef(typ Type, path BundleRefPath, allowed, atRoot bool) (BundleRefPath, bool) {
	switch t := typ.(type) {
	case *BundleType:
		if !allowed {
			return path, true
		}

	case *ListType:
		return findUnsupportedBundleRef(t.ValueType, path.Index(0), allowed && atRoot, false)

	case *SetType:
		return findUnsupportedBundleRef(t.ValueType, path.Index(0), allowed && atRoot, false)

	case *MapType:
		return findUnsupportedBundleRef(t.ValueType, path.AnyKey(), false, false)

	case *TupleType:
		for i, elem := range t.Elems {
			if p, found := findUnsupportedBundleRef(elem, path.Index(i), false, false); found {
				return p, true
			}
		}

	case *VariantType:
		for _, opt := range t.Options {
			if p, found := findUnsupportedBundleRef(opt, path, false, false); found {
				return p, true
			}
		}

	case *NonStrictType:
		return findUnsupportedBundleRef(t.Inner, path, allowed, atRoot)

	case *ObjectType:
		for _, attr := range t.Attributes {
			if p, found := findUnsupportedBundleRef(attr.Type, path.Attr(attr.Name), true, true); found {
				return p, true
			}
		}

	case *MergedObjectType:
		for _, obj := range t.Objects {
			if p, found := findUnsupportedBundleRef(obj, path, allowed, atRoot); found {
				return p, true
			}
		}
	}

	return nil, false
}
