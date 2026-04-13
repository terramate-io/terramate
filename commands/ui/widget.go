// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/typeschema"
)

// InputWidget is the interface for composable input controls.
// Each widget encapsulates the prepare/update/render/value lifecycle for a
// specific input type. Container widgets (list, map, object) hold child
// widgets, enabling recursive type composition.
//
// Widgets commit their value by calling ctx.UpdateValue() which writes the
// value into the WidgetContext and syncs it into the eval context for
// cross-input references. If validation fails the widget should store the
// error and render it, returning WidgetContinue.
type InputWidget interface {
	// WidgetContext returns the widget context.
	WidgetContext() *WidgetContext

	// Prepare initializes the widget for active editing.
	Prepare()

	// Update handles a key event. Returns a signal and optional tea.Cmd.
	// When returning WidgetConfirmed, the widget must have already called
	// ctx.UpdateValue() successfully.
	Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd)

	// Render returns the lines for the active input area.
	Render() []string

	// FormatDisplay returns a short summary for the completed inputs panel.
	FormatDisplay() string

	// ForwardMsg forwards non-key messages (e.g. cursor blink) to internal controls.
	ForwardMsg(msg tea.Msg) tea.Cmd

	// AcceptSubFormResult receives the result from a completed sub-form.
	// Returns true if the widget is fully done (caller should confirm/advance),
	// or false if the widget stays active (e.g. list widget awaiting more items).
	AcceptSubFormResult(result SubFormResult) bool
}

// SubFormResult carries the output of a completed sub-form back to its
// parent widget.
type SubFormResult struct {
	Values map[string]cty.Value
}

// WidgetSignal communicates intent from a widget back to InputsForm.
type WidgetSignal int

// WidgetContinue and the following constants define the signals a widget can return.
const (
	WidgetContinue    WidgetSignal = iota // Normal operation, keep editing
	WidgetConfirmed                       // Value accepted, advance to next input
	WidgetBack                            // User pressed Esc/ShiftTab, go back
	WidgetNeedSubForm                     // Container needs to open a nested form
)

// BundleOption is a unified representation of a selectable bundle for the
// bundle-ref widget, covering both existing on-disk bundles and session-created ones.
type BundleOption struct {
	Name  string
	Alias string
	EnvID string
}

// SharedWidgetContext provides shared resources to widgets.
type SharedWidgetContext struct {
	Schemactx typeschema.EvalContext
	Registry  *config.Registry
	Env       *config.Environment
	FromEnv   *config.Environment
}

// WidgetContext provides per-input context including the shared resources, display width, and current value.
type WidgetContext struct {
	*SharedWidgetContext
	Width int
	Def   *config.InputDefinition
	Value cty.Value
}

// SubFormRequest carries metadata when a widget signals WidgetNeedSubForm.
type SubFormRequest struct {
	InputDefs   []*config.InputDefinition // Sub-inputs for the nested form
	InputID     string                    // ID of the parent input being edited
	Title       string                    // Contextual title suffix for the subform (e.g. "Add item 3")
	EditMode    bool                      // true = edit existing values, false = set new
	Values      map[string]cty.Value      // Existing values when EditMode is true
	RefClass    string                    // Non-empty for bundle-ref creation
	SingleInput bool                      // When true, the sub-form hides the bottom panel
}

// NewWidget creates the appropriate InputWidget for a typeschema.Type.
func NewWidget(wctx *WidgetContext, typ typeschema.Type) InputWidget {
	if wctx.Def.HasPromptOptions() {
		if wctx.Def.Prompt.Multiselect {
			return NewMultiSelectWidget(wctx)
		}
		return NewSelectWidget(wctx)
	}

	switch t := typ.(type) {
	case *typeschema.PrimitiveType:
		switch t.Name {
		case "string":
			if wctx.Def.Prompt.Multiline {
				return NewMultilineWidget(wctx)
			}
			return NewTextWidget(wctx, t)
		case "bool":
			return NewBoolWidget(wctx)
		case "number":
			return NewTextWidget(wctx, t)
		}
	case *typeschema.ListType:
		return NewListWidget(wctx, t.ValueType)
	case *typeschema.SetType:
		return NewListWidget(wctx, t.ValueType)
	case *typeschema.MapType:
		return NewMapWidget(wctx, t.ValueType)
	case *typeschema.ObjectType:
		if len(t.Attributes) == 0 {
			return NewInlineMapWidget(wctx, &typeschema.PrimitiveType{Name: "string"})
		}
		return NewObjectWidget(wctx, t)
	case *typeschema.BundleType:
		return NewBundleRefWidget(wctx, t.ClassID)
	case *typeschema.NonStrictType:
		return NewWidget(wctx, t.Inner)
	}
	// Will crash.
	return nil
}

// isStructuredObject returns true if typ is an ObjectType with at least one
// defined attribute. An ObjectType with no attributes is treated as a free-form
// map(string) instead.
func isStructuredObject(typ typeschema.Type) bool {
	obj, ok := typ.(*typeschema.ObjectType)
	return ok && len(obj.Attributes) > 0
}

// NewListWidget is a helper to select the appropriate list widget sub-type.
func NewListWidget(wctx *WidgetContext, valTyp typeschema.Type) InputWidget {
	switch t := valTyp.(type) {
	case *typeschema.PrimitiveType:
		switch t.Name {
		case "string", "number":
			return NewInlineListWidget(wctx, t)
		}
	}
	return NewSubFormListWidget(wctx, valTyp)
}

// NewMapWidget is a helper to select the appropriate map widget sub-type.
func NewMapWidget(wctx *WidgetContext, valTyp typeschema.Type) InputWidget {
	switch t := valTyp.(type) {
	case *typeschema.PrimitiveType:
		switch t.Name {
		case "string", "number":
			return NewInlineMapWidget(wctx, t)
		}
	}
	return NewSubFormMapWidget(wctx, valTyp)
}

// UpdateValue writes val into the WidgetContext and syncs it to the eval
// context so that cross-input references see the new value.
func (ctx *WidgetContext) UpdateValue(val cty.Value) {
	ctx.Value = val
	syncInputToEvalctx(ctx.Schemactx.Evalctx, ctx.Def.Name, val)
}

// syncInputToEvalctx updates bundle.input.<name>.value in the eval context.
// If val is NilVal, the value is cleared.
func syncInputToEvalctx(evalctx *eval.Context, name string, val cty.Value) {
	var bundleVals map[string]cty.Value
	if ns, ok := evalctx.GetNamespace("bundle"); ok {
		bundleVals = ns.AsValueMap()
	}
	if bundleVals == nil {
		bundleVals = map[string]cty.Value{}
	}

	var inputVals map[string]cty.Value
	if existing, ok := bundleVals["input"]; ok {
		inputVals = existing.AsValueMap()
	}
	if inputVals == nil {
		inputVals = map[string]cty.Value{}
	}

	if val != cty.NilVal {
		inputVals[name] = cty.ObjectVal(map[string]cty.Value{"value": val})
	} else {
		delete(inputVals, name)
	}

	bundleVals["input"] = cty.ObjectVal(inputVals)
	evalctx.SetNamespace("bundle", bundleVals)
}

// FormatDisplayValue renders a one-line display string for a cty.Value using
// the typeschema.Type to provide richer formatting for complex types.
func FormatDisplayValue(val cty.Value, typ typeschema.Type) string {
	if val == cty.NilVal || val.IsNull() {
		return ""
	}

	switch t := typ.(type) {
	case *typeschema.PrimitiveType:
		return ctyToDisplayString(val)

	case *typeschema.ListType:
		return formatCollectionDisplay(val, t.ValueType)

	case *typeschema.SetType:
		return formatCollectionDisplay(val, t.ValueType)

	case *typeschema.TupleType:
		if !val.CanIterateElements() {
			return "<empty>"
		}
		n := val.LengthInt()
		if n == 0 {
			return "<empty>"
		}
		if n == 1 {
			it := val.ElementIterator()
			it.Next()
			_, elem := it.Element()
			if len(t.Elems) > 0 {
				return FormatDisplayValue(elem, t.Elems[0])
			}
			return ctyToDisplayString(elem)
		}
		return fmt.Sprintf("<%d items>", n)

	case *typeschema.MapType:
		if !val.CanIterateElements() {
			return "<empty>"
		}
		n := val.LengthInt()
		if n == 0 {
			return "<empty>"
		}
		if n == 1 {
			it := val.ElementIterator()
			it.Next()
			k, v := it.Element()
			return fmt.Sprintf("%s = %s", k.AsString(), FormatDisplayValue(v, t.ValueType))
		}
		return fmt.Sprintf("<%d entries>", n)

	case *typeschema.ObjectType:
		if !val.CanIterateElements() {
			return "<object>"
		}
		for _, attr := range t.Attributes {
			if val.Type().HasAttribute(attr.Name) {
				v := val.GetAttr(attr.Name)
				display := fmt.Sprintf("%s = %s", attr.Name, FormatDisplayValue(v, attr.Type))
				if len(t.Attributes) > 1 {
					return display + ", ..."
				}
				return display
			}
		}
		return "<object>"

	case *typeschema.NonStrictType:
		return FormatDisplayValue(val, t.Inner)

	default:
		return ctyToDisplayString(val)
	}
}

func formatCollectionDisplay(val cty.Value, elemType typeschema.Type) string {
	if !val.CanIterateElements() {
		return "<empty>"
	}
	n := val.LengthInt()
	if n == 0 {
		return "<empty>"
	}
	if n == 1 {
		it := val.ElementIterator()
		it.Next()
		_, elem := it.Element()
		return FormatDisplayValue(elem, elemType)
	}
	return fmt.Sprintf("<%d items>", n)
}
