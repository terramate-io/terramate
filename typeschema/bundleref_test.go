// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package typeschema

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

// resolved fakes what tm_bundle returns for a key: bundles of the same class
// still have different cty types, which is why collections must be rebuilt as
// tuples and objects.
func resolved(key string) cty.Value {
	attrs := map[string]cty.Value{
		"alias": cty.StringVal(key),
		"class": cty.StringVal("test.class/v1"),
	}
	// Vary the object type per key, like real bundles do via their inputs.
	attrs["input"] = cty.ObjectVal(map[string]cty.Value{key: cty.True})
	return cty.ObjectVal(attrs)
}

// resolveAll is a BundleRefFunc that resolves every string key and records the
// paths it was called at.
func resolveAll(paths *[]string) BundleRefFunc {
	return func(_ *BundleType, val cty.Value, path BundleRefPath) (cty.Value, error) {
		*paths = append(*paths, path.String())
		if val.IsNull() || val.Type() != cty.String {
			return val, nil
		}
		return resolved(val.AsString()), nil
	}
}

// The walker deliberately has no opinion on which shapes an input may use -
// that is [ValidateBundleRefPositions]' job - so it is exercised on shapes a
// definition would be rejected for, to keep it correct if the restriction is
// ever lifted.
func TestMapBundleRefsPaths(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		typeStr   string
		val       cty.Value
		wantPaths []string
	}{
		{
			name:      "top level bundle",
			typeStr:   `bundle("test.class/v1")`,
			val:       cty.StringVal("a"),
			wantPaths: []string{""},
		},
		{
			name:      "list of bundles",
			typeStr:   `list(bundle("test.class/v1"))`,
			val:       cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")}),
			wantPaths: []string{"[0]", "[1]"},
		},
		{
			name:      "set of bundles",
			typeStr:   `set(bundle("test.class/v1"))`,
			val:       cty.TupleVal([]cty.Value{cty.StringVal("a")}),
			wantPaths: []string{"[0]"},
		},
		{
			name:    "map of bundles",
			typeStr: `map(bundle("test.class/v1"))`,
			val: cty.ObjectVal(map[string]cty.Value{
				"primary": cty.StringVal("a"),
			}),
			wantPaths: []string{`["primary"]`},
		},
		{
			name:      "tuple of bundle and string",
			typeStr:   `tuple(bundle("test.class/v1"), string)`,
			val:       cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("not-a-ref")}),
			wantPaths: []string{"[0]"},
		},
		{
			name:      "nested list of lists",
			typeStr:   `list(list(bundle("test.class/v1")))`,
			val:       cty.TupleVal([]cty.Value{cty.TupleVal([]cty.Value{cty.StringVal("a")})}),
			wantPaths: []string{"[0][0]"},
		},
		{
			name:      "cty list value instead of tuple",
			typeStr:   `list(bundle("test.class/v1"))`,
			val:       cty.ListVal([]cty.Value{cty.StringVal("a")}),
			wantPaths: []string{"[0]"},
		},
		{
			name:      "empty list is not walked",
			typeStr:   `list(bundle("test.class/v1"))`,
			val:       cty.EmptyTupleVal,
			wantPaths: nil,
		},
		{
			name:      "null value is not walked",
			typeStr:   `list(bundle("test.class/v1"))`,
			val:       cty.NullVal(cty.List(cty.String)),
			wantPaths: nil,
		},
		{
			name:      "no bundle type in tree",
			typeStr:   `list(string)`,
			val:       cty.TupleVal([]cty.Value{cty.StringVal("a")}),
			wantPaths: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			typ, err := Parse(tc.typeStr, nil)
			assert.NoError(t, err)

			var paths []string
			_, err = MapBundleRefs(typ, tc.val, EvalContext{}, resolveAll(&paths))
			assert.NoError(t, err)
			assert.EqualStrings(t, sprintStrings(tc.wantPaths), sprintStrings(paths))
		})
	}
}

func TestMapBundleRefsRebuildsCollectionsAsTuples(t *testing.T) {
	t.Parallel()

	typ, err := Parse(`list(bundle("test.class/v1"))`, nil)
	assert.NoError(t, err)

	// Both elements resolve to objects with *different* cty types, so the result
	// can only be a tuple. cty.ListVal would panic here.
	val := cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})

	var paths []string
	got, err := MapBundleRefs(typ, val, EvalContext{}, resolveAll(&paths))
	assert.NoError(t, err)

	assert.IsTrue(t, got.Type().IsTupleType(), "expected tuple, got %s", got.Type().FriendlyName())
	assert.EqualInts(t, 2, got.LengthInt())

	elems := got.AsValueSlice()
	assert.EqualStrings(t, "a", elems[0].GetAttr("alias").AsString())
	assert.EqualStrings(t, "b", elems[1].GetAttr("alias").AsString())
}

func TestMapBundleRefsKeepsUnrelatedValues(t *testing.T) {
	t.Parallel()

	// A type tree without any bundle position must hand back the identical value.
	typ, err := Parse(`map(list(string))`, nil)
	assert.NoError(t, err)

	val := cty.ObjectVal(map[string]cty.Value{
		"a": cty.TupleVal([]cty.Value{cty.StringVal("x")}),
	})

	got, err := MapBundleRefs(typ, val, EvalContext{}, func(_ *BundleType, v cty.Value, _ BundleRefPath) (cty.Value, error) {
		t.Fatal("callback must not be invoked")
		return v, nil
	})
	assert.NoError(t, err)
	assert.IsTrue(t, got.RawEquals(val), "value was rebuilt unnecessarily")
}

func TestMapBundleRefsInObjectAttributes(t *testing.T) {
	t.Parallel()

	typ, err := Parse("object", []*ObjectTypeAttribute{
		{Name: "team", Type: &BundleType{ClassID: "test.class/v1"}},
		{Name: "peers", Type: &ListType{ValueType: &BundleType{ClassID: "test.class/v1"}}},
		{Name: "name", Type: &PrimitiveType{Name: "string"}},
	})
	assert.NoError(t, err)

	val := cty.ObjectVal(map[string]cty.Value{
		"team":  cty.StringVal("a"),
		"peers": cty.TupleVal([]cty.Value{cty.StringVal("b")}),
		"name":  cty.StringVal("keep-me"),
		// Not covered by the type: must survive the rebuild.
		"extra": cty.StringVal("untouched"),
	})

	var paths []string
	got, err := MapBundleRefs(typ, val, EvalContext{}, resolveAll(&paths))
	assert.NoError(t, err)

	assert.EqualStrings(t, sprintStrings([]string{".team", ".peers[0]"}), sprintStrings(paths))
	assert.EqualStrings(t, "a", got.GetAttr("team").GetAttr("alias").AsString())
	assert.EqualStrings(t, "b", got.GetAttr("peers").AsValueSlice()[0].GetAttr("alias").AsString())
	assert.EqualStrings(t, "keep-me", got.GetAttr("name").AsString())
	assert.EqualStrings(t, "untouched", got.GetAttr("extra").AsString())
}

func TestMapBundleRefsThroughSchemaReference(t *testing.T) {
	t.Parallel()

	schemas := NewSchemaNamespaces()
	schemas.Set("my", []*Schema{{
		Name: "Team",
		Type: &ObjectType{Attributes: []*ObjectTypeAttribute{
			{Name: "ref", Type: &BundleType{ClassID: "test.class/v1"}},
		}},
	}})
	schemactx := EvalContext{Schemas: schemas}

	typ, err := Parse("list(my.Team)", nil)
	assert.NoError(t, err)

	val := cty.TupleVal([]cty.Value{
		cty.ObjectVal(map[string]cty.Value{"ref": cty.StringVal("a")}),
	})

	var paths []string
	got, err := MapBundleRefs(typ, val, schemactx, resolveAll(&paths))
	assert.NoError(t, err)

	assert.EqualStrings(t, sprintStrings([]string{"[0].ref"}), sprintStrings(paths))
	assert.EqualStrings(t, "a", got.AsValueSlice()[0].GetAttr("ref").GetAttr("alias").AsString())
}

func TestMapBundleRefsPropagatesError(t *testing.T) {
	t.Parallel()

	typ, err := Parse(`list(bundle("test.class/v1"))`, nil)
	assert.NoError(t, err)

	val := cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})

	calls := 0
	_, err = MapBundleRefs(typ, val, EvalContext{}, func(_ *BundleType, v cty.Value, path BundleRefPath) (cty.Value, error) {
		calls++
		if path.String() == "[1]" {
			return v, errors.E("boom")
		}
		return v, nil
	})
	assert.Error(t, err)
	assert.EqualInts(t, 2, calls)
}

func sprintStrings(s []string) string {
	return fmt.Sprintf("%q", s)
}

// A bundle reference resolves at any depth, but only two shapes have an editor
// that makes sense, so the rest is rejected at definition time.
func TestValidateBundleRefPositions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		typeStr string
		wantErr bool
	}{
		{typeStr: `bundle("test.class/v1")`},
		{typeStr: `list(bundle("test.class/v1"))`},
		{typeStr: `set(bundle("test.class/v1"))`},
		{typeStr: `list(string)`},
		{typeStr: `map(object)`},

		{typeStr: `map(bundle("test.class/v1"))`, wantErr: true},
		{typeStr: `list(list(bundle("test.class/v1")))`, wantErr: true},
		{typeStr: `set(set(bundle("test.class/v1")))`, wantErr: true},
		{typeStr: `list(map(bundle("test.class/v1")))`, wantErr: true},
		{typeStr: `tuple(bundle("test.class/v1"), string)`, wantErr: true},
		{typeStr: `any_of(string, bundle("test.class/v1"))`, wantErr: true},
	} {
		t.Run(tc.typeStr, func(t *testing.T) {
			t.Parallel()

			typ, err := Parse(tc.typeStr, nil)
			if err != nil {
				t.Fatalf("parsing %q: %v", tc.typeStr, err)
			}
			err = ValidateBundleRefPositions(typ)
			if tc.wantErr && err == nil {
				t.Fatalf("%s: expected an error", tc.typeStr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.typeStr, err)
			}
		})
	}
}

// An attribute carries a type of its own, so the supported shapes are supported
// there too - including inside a list of objects, which the form edits with one
// sub-form per item and the usual picker inside it.
func TestValidateBundleRefPositionsInObjectAttributes(t *testing.T) {
	t.Parallel()

	attrType := func(t *testing.T, typeStr string) Type {
		t.Helper()
		typ, err := Parse(typeStr, nil)
		if err != nil {
			t.Fatalf("parsing %q: %v", typeStr, err)
		}
		return typ
	}

	for _, tc := range []struct {
		typeStr string
		wantErr bool
	}{
		{typeStr: `bundle("test.class/v1")`},
		{typeStr: `list(bundle("test.class/v1"))`},
		{typeStr: `map(bundle("test.class/v1"))`, wantErr: true},
	} {
		t.Run(tc.typeStr, func(t *testing.T) {
			t.Parallel()

			obj := &ObjectType{Attributes: []*ObjectTypeAttribute{
				{Name: "team", Type: attrType(t, tc.typeStr)},
			}}
			for name, typ := range map[string]Type{
				"attribute":       obj,
				"list(attribute)": &ListType{ValueType: obj},
			} {
				err := ValidateBundleRefPositions(typ)
				if tc.wantErr && err == nil {
					t.Fatalf("%s in %s: expected an error", tc.typeStr, name)
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("%s in %s: unexpected error: %v", tc.typeStr, name, err)
				}
			}
		})
	}
}

// A value whose shape does not match its type is the caller's problem, not the
// walker's - [Type.Apply] reports it. The walker has to hand such a value back
// untouched, which the sequence cases have to guard for explicitly: objects and
// maps iterate too, and AsValueSlice drops their keys instead of failing.
func TestMapBundleRefsLeavesMismatchedShapesUntouched(t *testing.T) {
	t.Parallel()

	bt := &BundleType{ClassID: "test.class/v1"}

	for _, tc := range []struct {
		name string
		typ  Type
		val  cty.Value
	}{
		{
			name: "list type over an object value",
			typ:  &ListType{ValueType: bt},
			val: cty.ObjectVal(map[string]cty.Value{
				"primary": cty.StringVal("a"),
				"backup":  cty.StringVal("b"),
			}),
		},
		{
			name: "set type over a map value",
			typ:  &SetType{ValueType: bt},
			val:  cty.MapVal(map[string]cty.Value{"primary": cty.StringVal("a")}),
		},
		{
			name: "tuple type over an object value",
			typ:  &TupleType{Elems: []Type{bt, &PrimitiveType{Name: "string"}}},
			val: cty.ObjectVal(map[string]cty.Value{
				"primary": cty.StringVal("a"),
				"label":   cty.StringVal("b"),
			}),
		},
		{
			name: "map type over a tuple value",
			typ:  &MapType{ValueType: bt},
			val:  cty.TupleVal([]cty.Value{cty.StringVal("a")}),
		},
		{
			name: "object type over a tuple value",
			typ: &ObjectType{Attributes: []*ObjectTypeAttribute{
				{Name: "primary", Type: bt},
			}},
			val: cty.TupleVal([]cty.Value{cty.StringVal("a")}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			out, err := MapBundleRefs(tc.typ, tc.val, EvalContext{Schemas: NewSchemaNamespaces()},
				func(_ *BundleType, _ cty.Value, _ BundleRefPath) (cty.Value, error) {
					called = true
					return cty.StringVal("resolved"), nil
				})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if called {
				t.Error("a mismatched shape must not be walked as if it matched")
			}
			if !out.RawEquals(tc.val) {
				t.Errorf("value was rewritten:\n got %#v\nwant %#v", out, tc.val)
			}
		})
	}
}
