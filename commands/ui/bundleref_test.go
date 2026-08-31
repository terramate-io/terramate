// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/typeschema"
)

// resolvedBundle mimics the object tm_bundle returns. Bundles of the same class
// have different cty types because their inputs differ.
func resolvedBundle(alias string) cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"alias": cty.StringVal(alias),
		"class": cty.StringVal("test.class/v1"),
		"uuid":  cty.NullVal(cty.String),
		"input": cty.ObjectVal(map[string]cty.Value{alias: cty.True}),
	})
}

func bundleRefListInput(name string) *config.InputDefinition {
	return &config.InputDefinition{
		Name:        name,
		Description: "Referenced bundles",
		Type:        &typeschema.ListType{ValueType: &typeschema.BundleType{ClassID: "test.class/v1"}},
		Prompt:      config.PromptConfig{Text: "Referenced bundles"},
	}
}

func TestGenerateBundleYAMLWritesBundleRefListAsKeys(t *testing.T) {
	change := Change{
		Kind:      ChangeCreate,
		Name:      "example",
		UUID:      "6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55",
		Source:    "/bundles/example/v1",
		InputDefs: []*config.InputDefinition{bundleRefListInput("teams")},
		UserValues: map[string]cty.Value{
			// Resolved objects, as they are held internally after evaluation.
			"teams": cty.TupleVal([]cty.Value{
				resolvedBundle("platform"),
				resolvedBundle("payments"),
			}),
		},
	}

	assertBundleYAML(t, change, nil, nil, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Referenced bundles
    teams:
      - platform
      - payments
`)
}

func TestGenerateBundleYAMLWritesNestedBundleRefAsKey(t *testing.T) {
	// A bundle reference inside an object attribute of a list item.
	itemType := &typeschema.ObjectType{Attributes: []*typeschema.ObjectTypeAttribute{
		{Name: "team", Type: &typeschema.BundleType{ClassID: "test.class/v1"}},
		{Name: "role", Type: &typeschema.PrimitiveType{Name: "string"}},
	}}

	change := Change{
		Kind:   ChangeCreate,
		Name:   "example",
		UUID:   "6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55",
		Source: "/bundles/example/v1",
		InputDefs: []*config.InputDefinition{{
			Name:        "grants",
			Description: "Team grants",
			Type:        &typeschema.ListType{ValueType: itemType},
			Prompt:      config.PromptConfig{Text: "Team grants"},
		}},
		UserValues: map[string]cty.Value{
			"grants": cty.TupleVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"team": resolvedBundle("platform"),
					"role": cty.StringVal("admin"),
				}),
			}),
		},
	}

	assertBundleYAML(t, change, nil, nil, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Team grants
    grants:
      - role: admin
        team: platform
`)
}

func TestNormalizeBundleRefValuesReducesCollectionsToKeys(t *testing.T) {
	defs := []*config.InputDefinition{
		bundleRefListInput("teams"),
		{
			Name: "owner",
			Type: &typeschema.BundleType{ClassID: "test.class/v1"},
		},
		{
			Name: "labels",
			Type: &typeschema.ListType{ValueType: &typeschema.PrimitiveType{Name: "string"}},
		},
	}

	values := map[string]cty.Value{
		"teams": cty.TupleVal([]cty.Value{
			resolvedBundle("platform"),
			resolvedBundle("payments"),
		}),
		"owner":  resolvedBundle("platform"),
		"labels": cty.TupleVal([]cty.Value{cty.StringVal("a")}),
	}

	normalizeBundleRefValues(typeschema.EvalContext{}, defs, values)

	want := cty.TupleVal([]cty.Value{cty.StringVal("platform"), cty.StringVal("payments")})
	if !values["teams"].RawEquals(want) {
		t.Fatalf("teams: got %#v, want %#v", values["teams"], want)
	}
	if !values["owner"].RawEquals(cty.StringVal("platform")) {
		t.Fatalf("owner: got %#v", values["owner"])
	}
	if !values["labels"].RawEquals(cty.TupleVal([]cty.Value{cty.StringVal("a")})) {
		t.Fatalf("labels was rewritten: %#v", values["labels"])
	}
}

func TestCheckBundleRefsResolvedNamesUnresolvedElement(t *testing.T) {
	defs := []*config.InputDefinition{bundleRefListInput("teams")}

	t.Run("all resolved", func(t *testing.T) {
		values := map[string]cty.Value{
			"teams": cty.TupleVal([]cty.Value{resolvedBundle("platform")}),
		}
		if err := checkBundleRefsResolved(typeschema.EvalContext{}, defs, values); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one element missing", func(t *testing.T) {
		values := map[string]cty.Value{
			"teams": cty.TupleVal([]cty.Value{
				resolvedBundle("platform"),
				cty.NullVal(cty.DynamicPseudoType),
			}),
		}
		err := checkBundleRefsResolved(typeschema.EvalContext{}, defs, values)
		if err == nil {
			t.Fatal("expected an error for the unresolved element")
		}
		if !strings.Contains(err.Error(), `"teams[1]"`) {
			t.Fatalf("error does not name the element: %s", err)
		}
	})

	t.Run("several elements missing", func(t *testing.T) {
		values := map[string]cty.Value{
			"teams": cty.TupleVal([]cty.Value{
				cty.NullVal(cty.DynamicPseudoType),
				resolvedBundle("platform"),
				cty.NullVal(cty.DynamicPseudoType),
			}),
		}
		err := checkBundleRefsResolved(typeschema.EvalContext{}, defs, values)
		if err == nil {
			t.Fatal("expected an error for the unresolved elements")
		}
		if !strings.Contains(err.Error(), `"teams[0]", "teams[2]"`) {
			t.Fatalf("error does not name both elements: %s", err)
		}
	})

	t.Run("top level miss keeps naming the input", func(t *testing.T) {
		singleDefs := []*config.InputDefinition{{
			Name: "owner",
			Type: &typeschema.BundleType{ClassID: "test.class/v1"},
		}}
		values := map[string]cty.Value{"owner": cty.NullVal(cty.DynamicPseudoType)}
		err := checkBundleRefsResolved(typeschema.EvalContext{}, singleDefs, values)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), `"owner"`) {
			t.Fatalf("unexpected error: %s", err)
		}
	})
}

func bundleRefListWidgetContext(t *testing.T, aliases ...string) *WidgetContext {
	t.Helper()

	reg := &config.Registry{}
	for _, alias := range aliases {
		reg.Bundles = append(reg.Bundles, &config.Bundle{
			DefinitionMetadata: config.Metadata{Class: "test.class/v1"},
			Alias:              alias,
			Name:               alias,
			Inputs:             map[string]cty.Value{},
			Exports:            map[string]cty.Value{},
		})
	}

	return &WidgetContext{
		SharedWidgetContext: &SharedWidgetContext{
			Schemactx: typeschema.EvalContext{
				Evalctx: eval.NewContext(map[string]function.Function{}),
				Schemas: typeschema.NewSchemaNamespaces(),
			},
			Registry: reg,
		},
		Def:   bundleRefListInput("teams"),
		Value: cty.NilVal,
	}
}

func TestBundleRefListWidgetSeedsSelectionFromResolvedValue(t *testing.T) {
	wctx := bundleRefListWidgetContext(t, "platform", "payments")
	// Reconfigure seeds the form with keys, but be tolerant of resolved objects.
	wctx.Value = cty.TupleVal([]cty.Value{resolvedBundle("payments")})

	w := NewBundleRefListWidget(wctx, "test.class/v1")
	w.Prepare()

	if got := w.FormatDisplay(); got != "payments" {
		t.Fatalf("unexpected display: %q", got)
	}
	if !w.isSelected("payments") || w.isSelected("platform") {
		t.Fatalf("unexpected selection: %v", w.selected)
	}
}

func TestBundleRefListWidgetTogglesAndCommitsInSelectionOrder(t *testing.T) {
	wctx := bundleRefListWidgetContext(t, "platform", "payments")

	w := NewBundleRefListWidget(wctx, "test.class/v1")
	w.Prepare()

	// Select the second bundle first, then the first one.
	w.cursor = 1
	w.toggle(w.rows()[w.cursor].alias)
	w.cursor = 0
	w.toggle(w.rows()[w.cursor].alias)
	w.commit()

	want := cty.TupleVal([]cty.Value{cty.StringVal("payments"), cty.StringVal("platform")})
	if !wctx.Value.RawEquals(want) {
		t.Fatalf("got %#v, want %#v", wctx.Value, want)
	}

	// Toggling an alias off removes it and keeps the rest.
	w.toggle("payments")
	w.commit()
	want = cty.TupleVal([]cty.Value{cty.StringVal("platform")})
	if !wctx.Value.RawEquals(want) {
		t.Fatalf("got %#v, want %#v", wctx.Value, want)
	}
}

func TestBundleRefListWidgetKeepsEditingAfterInlineCreate(t *testing.T) {
	// The registry does not know the new bundle yet, so the widget has to list
	// the selected alias on its own.
	wctx := bundleRefListWidgetContext(t, "platform")

	w := NewBundleRefListWidget(wctx, "test.class/v1")
	w.Prepare()
	w.toggle("platform")

	done := w.AcceptCreatedBundleRef("brand-new")
	if done {
		t.Fatal("a collection of references must stay active for further additions")
	}

	want := cty.TupleVal([]cty.Value{cty.StringVal("platform"), cty.StringVal("brand-new")})
	if !wctx.Value.RawEquals(want) {
		t.Fatalf("got %#v, want %#v", wctx.Value, want)
	}
	rows := w.rows()
	if len(rows) != 2 || rows[1].alias != "brand-new" {
		t.Fatalf("unknown alias is not listed: %+v", rows)
	}
	if w.cursor != len(rows) {
		t.Fatalf("cursor should rest on \"Add new\", got %d", w.cursor)
	}
}

// The cursor runs over the bundles and ends on the "Add new" row. Enter confirms
// from a bundle row, the way the ordinary multi-select does; only "Add new" does
// something else, which is why the help hint follows the cursor.
func TestBundleRefListWidgetKeys(t *testing.T) {
	wctx := bundleRefListWidgetContext(t, "platform", "payments")

	w := NewBundleRefListWidget(wctx, "test.class/v1")
	w.Prepare()

	if got := w.HelpHints(); got != "enter: confirm • space: toggle" {
		t.Fatalf("unexpected hint on a bundle row: %q", got)
	}

	// Space toggles the row under the cursor without finishing the input.
	if signal, _ := w.Update(tea.KeyMsg{Type: tea.KeySpace}); signal != WidgetContinue {
		t.Fatalf("space should not confirm, got %v", signal)
	}
	if !w.isSelected("platform") {
		t.Fatal("space should toggle the row under the cursor")
	}

	// Enter on a bundle row confirms the whole selection.
	if signal, _ := w.Update(tea.KeyMsg{Type: tea.KeyEnter}); signal != WidgetConfirmed {
		t.Fatalf("expected enter on a bundle row to confirm, got %v", signal)
	}
	want := cty.TupleVal([]cty.Value{cty.StringVal("platform")})
	if !wctx.Value.RawEquals(want) {
		t.Fatalf("got %#v, want %#v", wctx.Value, want)
	}

	// Down past the last bundle reaches "Add new", where enter creates instead
	// of confirming and the hints drop away.
	for range 2 {
		w.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if got := w.HelpHints(); got != "" {
		t.Fatalf("the \"Add new\" row should show no hints, got %q", got)
	}
	if signal, _ := w.Update(tea.KeyMsg{Type: tea.KeyEnter}); signal != WidgetNeedSubForm {
		t.Fatalf("expected the \"Add new\" row to request a sub-form, got %v", signal)
	}

	// Down stops on the create row rather than running off the end.
	before := w.cursor
	w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.cursor != before {
		t.Fatalf("cursor ran past the \"Add new\" row: %d -> %d", before, w.cursor)
	}
}

func TestBundleRefListWidgetEmptySelectionCommitsEmptyTuple(t *testing.T) {
	wctx := bundleRefListWidgetContext(t, "platform")

	w := NewBundleRefListWidget(wctx, "test.class/v1")
	w.Prepare()
	w.commit()

	if !wctx.Value.RawEquals(cty.EmptyTupleVal) {
		t.Fatalf("got %#v", wctx.Value)
	}
	if got := w.FormatDisplay(); got != "<none>" {
		t.Fatalf("unexpected display: %q", got)
	}
}

// The completed-inputs panel has to name the referenced bundles; a list of
// resolved bundle objects would otherwise render as an opaque item count.
func TestFormatDisplayValueNamesBundleRefsInCollection(t *testing.T) {
	t.Parallel()

	typ, err := typeschema.Parse(`list(bundle("test.class/v1"))`, nil)
	if err != nil {
		t.Fatal(err)
	}
	val := cty.TupleVal([]cty.Value{
		resolvedBundle("platform"),
		cty.StringVal("payments"), // Not yet resolved, still a key.
	})

	if got := FormatDisplayValue(val, typ); got != "platform, payments" {
		t.Fatalf("unexpected display: %q", got)
	}
}
