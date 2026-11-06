// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/typeschema"
)

func selectTestCtx() typeschema.EvalContext {
	return typeschema.EvalContext{
		Evalctx: eval.NewContext(map[string]function.Function{}),
		Schemas: typeschema.NewSchemaNamespaces(),
	}
}

func testAttr(t *testing.T, name, expr string) *ast.Attribute {
	t.Helper()

	e, err := ast.ParseExpression(expr, "/test.tm")
	if err != nil {
		t.Fatal(err)
	}
	attr := ast.NewAttribute("/", &hhcl.Attribute{Name: name, Expr: e, Range: e.Range()})
	return &attr
}

// multiselectInput builds an input that is prompted as a multi-select over the
// options "a" and "b".
func multiselectInput(t *testing.T, name string) *config.InputDefinition {
	t.Helper()

	attrs, err := config.EvalObjectAttributes(selectTestCtx().Evalctx, []*hcl.DefineObjectAttribute{{
		Name:        name,
		Description: testAttr(t, "description", `"Picked options"`),
		Type:        testAttr(t, "type", `list(string)`),
		Prompt: &hcl.DefineInputPrompt{
			Text:        testAttr(t, "text", `"Pick some"`),
			Options:     testAttr(t, "options", `["a", "b"]`),
			Multiselect: testAttr(t, "multiselect", `true`),
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return config.ObjectAttrToInputDef(attrs[0])
}

func TestMultiSelectWidgetEmptySelectionCommitsEmptyTuple(t *testing.T) {
	wctx := &WidgetContext{
		SharedWidgetContext: &SharedWidgetContext{Schemactx: selectTestCtx()},
		Def:                 multiselectInput(t, "picks"),
		Value:               cty.NilVal,
	}

	w := NewWidget(wctx, wctx.Def.Type)
	if _, ok := w.(*MultiSelectWidget); !ok {
		t.Fatalf("expected a multi-select widget, got %T", w)
	}
	w.Prepare()

	if signal, _ := w.Update(tea.KeyMsg{Type: tea.KeyEnter}); signal != WidgetConfirmed {
		t.Fatalf("expected enter to confirm, got %v", signal)
	}
	if !wctx.Value.RawEquals(cty.EmptyTupleVal) {
		t.Fatalf("got %#v, want an empty tuple", wctx.Value)
	}
	if got := w.FormatDisplay(); got != "<none>" {
		t.Fatalf("unexpected display: %q", got)
	}

	// Re-entering the input keeps the empty selection instead of falling
	// back to the default.
	w.Prepare()
	if got := w.(*MultiSelectWidget).selected; len(got) != 0 {
		t.Fatalf("unexpected selection: %v", got)
	}
}

// Confirming a multi-select with nothing selected is a valid answer, so the
// form must move on to the next input rather than prompting for it again.
func TestInputsFormAdvancesPastEmptyMultiSelect(t *testing.T) {
	defs := []*config.InputDefinition{
		multiselectInput(t, "picks"),
		promptedInput("name", "Name", "Name"),
	}

	f := NewInputsForm(defs, selectTestCtx(), &config.Registry{}, nil)
	if f.activeIdx != 0 {
		t.Fatalf("expected the multi-select to be active, got index %d", f.activeIdx)
	}

	f, _ = f.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if f.activeIdx != 1 {
		t.Fatalf("expected the form to advance to the next input, got index %d", f.activeIdx)
	}
	if got := f.UserValues()["picks"]; !got.RawEquals(cty.EmptyTupleVal) {
		t.Fatalf("got %#v, want an empty tuple", got)
	}

	// The empty selection is written out, rather than left to the default.
	assertBundleYAML(t, Change{
		Kind:       ChangeCreate,
		Name:       "example",
		UUID:       "6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55",
		Source:     "/bundles/example/v1",
		InputDefs:  defs,
		UserValues: f.UserValues(),
	}, nil, nil, `apiVersion: terramate.io/cli/v1
kind: BundleInstance
metadata:
  name: example
  uuid: 6f2c1f6a-5c1a-4a2e-9f1b-2a7c0d3e4b55
spec:
  source: /bundles/example/v1
  inputs:
    # tmdoc: Picked options
    picks: []
`)
}
