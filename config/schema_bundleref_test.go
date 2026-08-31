// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/typeschema"
)

const testClass = "test.io/team"

// bundleRefTestCtx builds an eval context with a tm_bundle backed by a registry
// holding the given aliases.
func bundleRefTestCtx(t *testing.T, aliases ...string) typeschema.EvalContext {
	t.Helper()

	reg := &Registry{}
	for _, alias := range aliases {
		reg.Bundles = append(reg.Bundles, &Bundle{
			DefinitionMetadata: Metadata{Class: testClass},
			Alias:              alias,
			Name:               alias,
			// Distinct input sets, so resolved bundles have distinct cty types.
			Inputs:  map[string]cty.Value{alias: cty.True},
			Exports: map[string]cty.Value{},
		})
	}

	evalctx := eval.NewContext(map[string]function.Function{})
	evalctx.SetFunction(stdlib.Name("bundle"), BundleFunc(context.Background(), reg, nil, false))
	return typeschema.EvalContext{Evalctx: evalctx, Schemas: typeschema.NewSchemaNamespaces()}
}

func TestResolveBundleRefsInCollections(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		typeStr string
		val     cty.Value
		// Aliases expected at each resolved position, in iteration order.
		wantAliases []string
	}{
		{
			name:        "single reference",
			typeStr:     `bundle("test.io/team")`,
			val:         cty.StringVal("platform"),
			wantAliases: []string{"platform"},
		},
		{
			name:        "list of references",
			typeStr:     `list(bundle("test.io/team"))`,
			val:         cty.TupleVal([]cty.Value{cty.StringVal("platform"), cty.StringVal("payments")}),
			wantAliases: []string{"platform", "payments"},
		},
		{
			name:        "set of references",
			typeStr:     `set(bundle("test.io/team"))`,
			val:         cty.TupleVal([]cty.Value{cty.StringVal("payments")}),
			wantAliases: []string{"payments"},
		},
		{
			name:    "list of [key, envID] tuples",
			typeStr: `list(bundle("test.io/team"))`,
			val: cty.TupleVal([]cty.Value{
				cty.TupleVal([]cty.Value{cty.StringVal("platform"), cty.StringVal("")}),
			}),
			wantAliases: []string{"platform"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sc := bundleRefTestCtx(t, "platform", "payments")
			typ, err := typeschema.Parse(tc.typeStr, nil)
			assert.NoError(t, err)

			applied, err := typ.Apply(tc.val, sc, true)
			assert.NoError(t, err)

			got, err := resolveBundleRefs(typ, applied, sc, "teams")
			assert.NoError(t, err)

			var aliases []string
			collect := func(v cty.Value) {
				assert.IsTrue(t, v.Type().IsObjectType(), "not resolved: %s", v.Type().FriendlyName())
				aliases = append(aliases, v.GetAttr("alias").AsString())
			}
			if got.Type().IsTupleType() {
				for _, elem := range got.AsValueSlice() {
					collect(elem)
				}
			} else {
				collect(got)
			}

			assert.EqualInts(t, len(tc.wantAliases), len(aliases))
			for i, want := range tc.wantAliases {
				assert.EqualStrings(t, want, aliases[i])
			}
		})
	}
}

func TestResolveBundleRefsYieldsNullForMissingElement(t *testing.T) {
	t.Parallel()

	sc := bundleRefTestCtx(t, "platform")
	typ, err := typeschema.Parse(`list(bundle("test.io/team"))`, nil)
	assert.NoError(t, err)

	val := cty.TupleVal([]cty.Value{cty.StringVal("platform"), cty.StringVal("gone")})
	applied, err := typ.Apply(val, sc, true)
	assert.NoError(t, err)

	got, err := resolveBundleRefs(typ, applied, sc, "teams")
	assert.NoError(t, err)

	elems := got.AsValueSlice()
	assert.EqualInts(t, 2, len(elems))
	assert.EqualStrings(t, "platform", elems[0].GetAttr("alias").AsString())
	assert.IsTrue(t, elems[1].IsNull(), "missing bundle must resolve to null, got %#v", elems[1])
}

func TestResolveBundleRefsLeavesUnrelatedTypesAlone(t *testing.T) {
	t.Parallel()

	sc := bundleRefTestCtx(t)
	typ, err := typeschema.Parse(`list(string)`, nil)
	assert.NoError(t, err)

	val := cty.TupleVal([]cty.Value{cty.StringVal("platform")})
	got, err := resolveBundleRefs(typ, val, sc, "labels")
	assert.NoError(t, err)
	assert.IsTrue(t, got.RawEquals(val), "value was modified: %#v", got)
}

func TestResolveBundleRefsPassesResolvedObjectsThrough(t *testing.T) {
	t.Parallel()

	// Values loaded from a bundle instance on disk are already resolved.
	sc := bundleRefTestCtx(t, "platform")
	typ, err := typeschema.Parse(`list(bundle("test.io/team"))`, nil)
	assert.NoError(t, err)

	resolved := MakeObjectFromBundle(&Bundle{
		DefinitionMetadata: Metadata{Class: testClass},
		Alias:              "platform",
		Inputs:             map[string]cty.Value{},
		Exports:            map[string]cty.Value{},
	})
	val := cty.TupleVal([]cty.Value{resolved})

	got, err := resolveBundleRefs(typ, val, sc, "teams")
	assert.NoError(t, err)
	assert.IsTrue(t, got.RawEquals(val), "resolved value was rewritten: %#v", got)
}

// typeAttr builds the `type = <expr>` attribute of an input or attribute
// definition from its source text.
func typeAttr(t *testing.T, typeStr string) *ast.Attribute {
	t.Helper()

	const filename = "/test.tm" // Absolute and under the rootdir, as ranges require.

	expr, diags := hclsyntax.ParseExpression([]byte(typeStr), filename, hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parsing %q: %v", typeStr, diags)
	}
	attr := ast.NewAttribute("/", &hhcl.Attribute{
		Name:  "type",
		Expr:  expr,
		Range: hhcl.Range{Filename: filename, Start: hhcl.InitialPos, End: hhcl.InitialPos},
	})
	return &attr
}

// The unsupported shapes have to be rejected where the definition is read, so
// the author sees the error instead of a form they cannot use.
func TestEvalInputSchemaRejectsUnsupportedBundleRefPositions(t *testing.T) {
	t.Parallel()

	evalctx := eval.NewContext(map[string]function.Function{})

	t.Run("input type", func(t *testing.T) {
		t.Parallel()

		_, err := EvalInputSchema(evalctx, &hcl.DefineInput{
			Name: "teams",
			Type: typeAttr(t, `map(bundle("test.io/team"))`),
		})
		if err == nil {
			t.Fatal("expected map(bundle(...)) to be rejected")
		}
		if !strings.Contains(err.Error(), "teams") {
			t.Fatalf("error does not name the input: %v", err)
		}
	})

	t.Run("attribute type", func(t *testing.T) {
		t.Parallel()

		_, err := EvalObjectAttributes(evalctx, []*hcl.DefineObjectAttribute{{
			Name: "team",
			Type: typeAttr(t, `list(list(bundle("test.io/team")))`),
		}})
		if err == nil {
			t.Fatal("expected list(list(bundle(...))) to be rejected")
		}
		if !strings.Contains(err.Error(), "team") {
			t.Fatalf("error does not name the attribute: %v", err)
		}
	})

	t.Run("supported shapes pass", func(t *testing.T) {
		t.Parallel()

		for _, typeStr := range []string{
			`bundle("test.io/team")`,
			`list(bundle("test.io/team"))`,
			`set(bundle("test.io/team"))`,
		} {
			if _, err := EvalInputSchema(evalctx, &hcl.DefineInput{
				Name: "teams",
				Type: typeAttr(t, typeStr),
			}); err != nil {
				t.Fatalf("%s: unexpected error: %v", typeStr, err)
			}
		}
	})
}
