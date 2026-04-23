// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/typeschema"
)

// InlineListWidget manages a list of items that are edited inline.
// This supports strings and numbers.
type InlineListWidget struct {
	wctx      *WidgetContext
	valueType typeschema.Type
	items     []cty.Value
	cursor    int // -1 = text input at bottom; >= 0 = index in items; len(items) = "Confirm"
	editing   int // -1 = not editing; >= 0 = editing item at this index
	textInput textinput.Model

	validationErr error
	numberMode    bool
}

// NewInlineListWidget creates a widget for editing a list of primitive values inline.
func NewInlineListWidget(wctx *WidgetContext, valueType typeschema.Type) *InlineListWidget {
	numberMode := valueType.String() == "number"

	ti := textinput.New()
	ti.Prompt = ""

	if numberMode {
		ti.CharLimit = 64
	} else {
		ti.CharLimit = 256
	}

	if wctx.Width > 12 {
		ti.Width = wctx.Width - 12
	}

	return &InlineListWidget{
		wctx:       wctx,
		valueType:  valueType,
		cursor:     -1,
		editing:    -1,
		textInput:  ti,
		numberMode: numberMode,
	}
}

// WidgetContext returns the widget's context.
func (w *InlineListWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *InlineListWidget) Prepare() {
	w.textInput.Reset()
	w.textInput.Focus()
	w.cursor = -1
	w.editing = -1

	val := w.wctx.Value
	if val == cty.NilVal {
		val, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	}
	w.setList(val)

	w.textInput.SetValue("")
	w.textInput.Placeholder = "Type to add"
}

// Update handles keyboard input and returns the resulting signal.
func (w *InlineListWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	n := len(w.items)

	if w.editing >= 0 {
		switch msg.Type {
		case tea.KeyEnter:
			w.validationErr = nil
			textVal := w.textInput.Value()
			if textVal != "" {
				var editVal cty.Value
				if w.numberMode {
					v, err := cty.ParseNumberVal(textVal)
					if err != nil {
						w.validationErr = errors.E("This field requires a number.")
						return WidgetContinue, nil
					}
					editVal = v
				} else {
					editVal = cty.StringVal(textVal)
				}
				w.items[w.editing] = editVal
			}
			// This still resets the value back to its pre-edit value.
			w.editing = -1
			w.cursor = -1
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Placeholder = "Type to add"
			return WidgetContinue, nil
		case tea.KeyEsc:
			w.editing = -1
			w.cursor = -1
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Placeholder = "Type to add"
			return WidgetContinue, nil
		default:
			w.validationErr = nil
			var cmd tea.Cmd
			w.textInput, cmd = w.textInput.Update(msg)
			return WidgetContinue, cmd
		}
	}

	// Cursor on "Confirm" (visual bottom)
	if w.cursor == n {
		switch msg.Type {
		case tea.KeyUp:
			w.cursor = -1
			w.textInput.Focus()
		case tea.KeyEnter:
			w.validationErr = nil

			w.wctx.UpdateValue(cty.TupleVal(w.items))
			return WidgetConfirmed, nil
		case tea.KeyShiftTab, tea.KeyEsc:
			return WidgetBack, nil
		}
		return WidgetContinue, nil
	}

	// Cursor on the add-new text field (visual middle, between items and Confirm)
	if w.cursor < 0 {
		switch msg.Type {
		case tea.KeyUp:
			if n > 0 {
				w.cursor = n - 1
				w.textInput.Blur()
			}
			return WidgetContinue, nil
		case tea.KeyDown:
			w.cursor = n
			w.textInput.Blur()
			return WidgetContinue, nil
		case tea.KeyEnter:
			w.validationErr = nil
			textVal := w.textInput.Value()
			if textVal != "" {
				var addVal cty.Value
				if w.numberMode {
					v, err := cty.ParseNumberVal(textVal)
					if err != nil {
						w.validationErr = errors.E("This field requires a number.")
						return WidgetContinue, nil
					}
					addVal = v
				} else {
					addVal = cty.StringVal(textVal)
				}

				w.items = append(w.items, addVal)
				w.textInput.Reset()
				w.textInput.SetValue("")
			}
			return WidgetContinue, nil
		case tea.KeyShiftTab, tea.KeyEsc:
			return WidgetBack, nil
		default:
			w.validationErr = nil
			var cmd tea.Cmd
			w.textInput, cmd = w.textInput.Update(msg)
			return WidgetContinue, cmd
		}
	}

	// Cursor on an existing item (visual top)
	switch msg.Type {
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n-1 {
			w.cursor++
		} else {
			w.cursor = -1
			w.textInput.Focus()
		}
	case tea.KeyEnter:
		w.editing = w.cursor
		w.textInput.Focus()
		w.textInput.SetValue(ctyToString(w.items[w.cursor]))
		w.textInput.Placeholder = ""
	case tea.KeyDelete, tea.KeyBackspace:
		w.items = append(w.items[:w.cursor], w.items[w.cursor+1:]...)
		if w.cursor >= len(w.items) {
			w.cursor = -1
			w.textInput.Focus()
		}
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *InlineListWidget) Render() []string {
	itemStyle := lipgloss.NewStyle().Foreground(colorText)
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	cursorItemStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)
	n := len(w.items)

	var lines []string

	for i, item := range w.items {
		if w.editing == i {
			lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), w.textInput.View()))
		} else if i == w.cursor {
			lines = append(lines, fmt.Sprintf("  %s %s  %s", promptStyle.Render("›"), cursorItemStyle.Render(ctyToString(item)), hintStyle.Render("enter: edit · delete: remove")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("-"), itemStyle.Render(ctyToString(item))))
		}
	}

	if w.editing < 0 {
		if w.cursor == -1 {
			lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("+"), w.textInput.View()))
		} else {
			lines = append(lines, dimStyle.Render("  + Add"))
		}
	}

	if w.validationErr != nil {
		lines = append(lines, validationStyle.Render(fmt.Sprintf("  %s", w.validationErr.Error())))
	}

	if w.cursor == n {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), cursorItemStyle.Render("Done")))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render(" "), dimStyle.Render("Done")))
	}

	return lines
}

func (w *InlineListWidget) setList(val cty.Value) {
	if val == cty.NilVal || val.IsNull() {
		w.items = nil
		return
	}
	if !val.CanIterateElements() {
		w.items = nil
		return
	}
	w.items = nil
	it := val.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		w.items = append(w.items, elem)
	}
}

// FormatDisplay returns a compact summary of the current list value.
func (w *InlineListWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() {
		return ""
	}
	if !val.CanIterateElements() {
		return ""
	}
	n := val.LengthInt()
	if n == 0 {
		return "<empty>"
	}
	if n == 1 {
		it := val.ElementIterator()
		it.Next()
		_, elem := it.Element()
		return ctyToDisplayString(elem)
	}
	return fmt.Sprintf("<%d items>", n)
}

// ForwardMsg forwards a bubbletea message to the underlying text input.
func (w *InlineListWidget) ForwardMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.textInput, cmd = w.textInput.Update(msg)
	return cmd
}

// AcceptSubFormResult is a no-op; inline lists do not use sub-forms.
func (w *InlineListWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// SubFormListWidget manages a list of complex items (objects, nested lists/maps,
// etc.) where each item is edited via a nested sub-form.
type SubFormListWidget struct {
	wctx      *WidgetContext
	valueType typeschema.Type
	items     []cty.Value
	cursor    int // 0..n-1 = items, n = "Add", n+1 = "Done"
	editIdx   int // -1 = adding new; >= 0 = editing existing item

	SubFormRequest *SubFormRequest
}

// NewSubFormListWidget creates a widget for editing a list of complex items via nested sub-forms.
func NewSubFormListWidget(wctx *WidgetContext, valueType typeschema.Type) InputWidget {
	return &SubFormListWidget{
		wctx:      wctx,
		valueType: valueType,
		editIdx:   -1,
	}
}

// WidgetContext returns the widget's context.
func (w *SubFormListWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *SubFormListWidget) Prepare() {
	w.SubFormRequest = nil
	w.editIdx = -1

	val := w.wctx.Value
	if val == cty.NilVal {
		val, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	}
	w.setList(val)
	w.cursor = len(w.items)
}

// Update handles keyboard input and returns the resulting signal.
func (w *SubFormListWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	n := len(w.items)

	switch msg.Type {
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n+1 {
			w.cursor++
		}
	case tea.KeyEnter:
		if w.cursor < n {
			w.editIdx = w.cursor
			w.SubFormRequest = w.buildSubFormRequest(true, w.items[w.cursor])
			w.SubFormRequest.Title = fmt.Sprintf("Edit item %d", w.cursor+1)
			return WidgetNeedSubForm, nil
		} else if w.cursor == n {
			w.editIdx = -1
			w.SubFormRequest = w.buildSubFormRequest(false, cty.NilVal)
			w.SubFormRequest.Title = fmt.Sprintf("Add item %d", n+1)
			return WidgetNeedSubForm, nil
		}
		w.commitList()
		return WidgetConfirmed, nil
	case tea.KeyDelete, tea.KeyBackspace:
		if w.cursor < n && n > 0 {
			w.items = append(w.items[:w.cursor], w.items[w.cursor+1:]...)
			if w.cursor >= len(w.items) {
				w.cursor = len(w.items)
			}
		}
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *SubFormListWidget) Render() []string {
	itemStyle := lipgloss.NewStyle().Foreground(colorText)
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	cursorItemStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)

	n := len(w.items)
	var lines []string

	for i, item := range w.items {
		summary := FormatDisplayValue(item, w.valueType)
		if w.cursor == i {
			lines = append(lines, fmt.Sprintf("  %s %s  %s", promptStyle.Render("›"), cursorItemStyle.Render(summary), hintStyle.Render("enter: edit · delete: remove")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("-"), itemStyle.Render(summary)))
		}
	}

	if w.cursor == n {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("+"), cursorItemStyle.Render("Add")))
	} else {
		lines = append(lines, dimStyle.Render("  + Add"))
	}

	if w.cursor == n+1 {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), cursorItemStyle.Render("Done")))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render(" "), dimStyle.Render("Done")))
	}

	return lines
}

// FormatDisplay returns a compact summary of the current list value.
// FormatDisplay may be called before Prepare(), so read from wctx.Value
// directly rather than the widget's edit buffer.
func (w *SubFormListWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() || !val.CanIterateElements() {
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
		return FormatDisplayValue(elem, w.valueType)
	}
	return fmt.Sprintf("<%d items>", n)
}

// ForwardMsg is a no-op; sub-form lists do not embed a text input.
func (w *SubFormListWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult integrates a completed sub-form result into the list.
func (w *SubFormListWidget) AcceptSubFormResult(result SubFormResult) bool {
	val := subFormResultToValue(w.valueType, result.Values)
	if val == cty.NilVal {
		return false
	}
	if w.editIdx >= 0 && w.editIdx < len(w.items) {
		w.items[w.editIdx] = val
	} else {
		w.items = append(w.items, val)
		w.cursor = len(w.items)
	}
	w.editIdx = -1
	return false
}

func (w *SubFormListWidget) setList(val cty.Value) {
	w.items = nil
	if val == cty.NilVal || val.IsNull() || !val.CanIterateElements() {
		return
	}
	it := val.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		w.items = append(w.items, elem)
	}
}

func (w *SubFormListWidget) commitList() {
	val := cty.EmptyTupleVal
	if len(w.items) > 0 {
		val = cty.TupleVal(w.items)
	}
	w.wctx.UpdateValue(val)
}

func (w *SubFormListWidget) buildSubFormRequest(editMode bool, existing cty.Value) *SubFormRequest {
	isObject := isStructuredObject(w.valueType)
	req := &SubFormRequest{
		InputID:     w.wctx.Def.Name,
		InputDefs:   inputDefsForType(w.valueType, w.wctx.Def.ObjectAttributes),
		EditMode:    editMode,
		SingleInput: !isObject,
	}
	if editMode && existing != cty.NilVal && !existing.IsNull() {
		req.Values = subFormValuesFromItem(w.valueType, existing)
	}
	return req
}

// inputDefsForType creates InputDefinitions for a sub-form that edits a single
// value of the given type. For objects with config-level attributes, each
// attribute becomes a separate input. For other types, a single synthetic
// "value" input is created.
func inputDefsForType(typ typeschema.Type, configAttrs []*config.ObjectAttribute) []*config.InputDefinition {
	if len(configAttrs) > 0 {
		return objectAttrsToInputDefs(configAttrs)
	}
	return []*config.InputDefinition{{
		Name: "value",
		Type: typ,
	}}
}

// subFormValuesFromItem extracts sub-form values from a cty.Value. For objects,
// each attribute is extracted separately. For other types, the whole value is
// wrapped under the "value" key.
func subFormValuesFromItem(typ typeschema.Type, val cty.Value) map[string]cty.Value {
	if isStructuredObject(typ) {
		return extractObjectAttrs(val)
	}
	return map[string]cty.Value{"value": val}
}

// subFormResultToValue converts sub-form results back into a single cty.Value.
// For objects, it builds an ObjectVal. For other types, it unwraps the "value" key.
func subFormResultToValue(typ typeschema.Type, values map[string]cty.Value) cty.Value {
	if isStructuredObject(typ) {
		m := make(map[string]cty.Value, len(values))
		for k, v := range values {
			if v != cty.NilVal {
				m[k] = v
			}
		}
		if len(m) == 0 {
			return cty.NilVal
		}
		return cty.ObjectVal(m)
	}
	if v, ok := values["value"]; ok && v != cty.NilVal {
		return v
	}
	return cty.NilVal
}

// ---------------------------------------------------------------------------
// Container helpers
// ---------------------------------------------------------------------------

// objectAttrsToInputDefs converts config-level ObjectAttributes to InputDefinitions for sub-forms.
func objectAttrsToInputDefs(attrs []*config.ObjectAttribute) []*config.InputDefinition {
	defs := make([]*config.InputDefinition, len(attrs))
	for i, attr := range attrs {
		defs[i] = config.ObjectAttrToInputDef(attr)
	}
	return defs
}

// extractObjectAttrs extracts attribute values from a cty object value.
func extractObjectAttrs(val cty.Value) map[string]cty.Value {
	if val == cty.NilVal || val.IsNull() || !val.CanIterateElements() {
		return nil
	}
	result := make(map[string]cty.Value)
	it := val.ElementIterator()
	for it.Next() {
		k, v := it.Element()
		result[k.AsString()] = v
	}
	return result
}
