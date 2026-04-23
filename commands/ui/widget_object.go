// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/zclconf/go-cty/cty"
)

// ObjectWidget manages a structured object with named fields.
// It signals WidgetNeedSubForm to open a nested form for editing the object's
// attributes, since objects use the full InputsForm for their sub-inputs.
type ObjectWidget struct {
	wctx    *WidgetContext
	objType *typeschema.ObjectType

	// SubFormRequest is populated when the widget signals WidgetNeedSubForm.
	SubFormRequest *SubFormRequest
}

// NewObjectWidget creates a widget for editing a structured object via a nested sub-form.
func NewObjectWidget(wctx *WidgetContext, objType *typeschema.ObjectType) *ObjectWidget {
	return &ObjectWidget{
		wctx:    wctx,
		objType: objType,
	}
}

// WidgetContext returns the widget's context.
func (w *ObjectWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *ObjectWidget) Prepare() {
	w.SubFormRequest = nil
}

// Update handles keyboard input and returns the resulting signal.
func (w *ObjectWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	val := w.wctx.Value
	hasValue := val != cty.NilVal && !val.IsNull()

	switch msg.Type {
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyEnter:
		req := &SubFormRequest{
			InputID:   w.wctx.Def.Name,
			InputDefs: objectAttrsToInputDefs(w.wctx.Def.ObjectAttributes),
			EditMode:  hasValue,
		}
		if req.EditMode {
			req.Values = extractObjectAttrs(val)
		}
		w.SubFormRequest = req
		return WidgetNeedSubForm, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *ObjectWidget) Render() []string {
	var lines []string

	val := w.wctx.Value
	hasValue := val != cty.NilVal && !val.IsNull()

	if hasValue && val.CanIterateElements() {
		dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
		valStyle := lipgloss.NewStyle().Foreground(colorSecondary)
		nameStyle := lipgloss.NewStyle().Foreground(colorText)

		maxN := 0
		for _, attr := range w.objType.Attributes {
			if len(attr.Name) > maxN {
				maxN = len(attr.Name)
			}
		}
		for _, attr := range w.objType.Attributes {
			pad := maxN - len(attr.Name)
			line := fmt.Sprintf("    %s%*s", nameStyle.Render(attr.Name), pad, "")
			if val.Type().HasAttribute(attr.Name) {
				v := val.GetAttr(attr.Name)
				line += " = " + valStyle.Render(FormatDisplayValue(v, attr.Type))
			} else {
				line += " " + dimStyle.Render("<not set>")
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	label := "Set values"
	if hasValue {
		label = "Edit values"
	}
	lines = append(lines, activeOptionStyle.Render("› "+label))
	return lines
}

// FormatDisplay returns a display string for the current object value.
func (w *ObjectWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() {
		return "<not set>"
	}
	return FormatDisplayValue(val, w.objType)
}

// ForwardMsg is a no-op; object widgets have no underlying input component.
func (w *ObjectWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult integrates a completed sub-form result into the object value.
func (w *ObjectWidget) AcceptSubFormResult(result SubFormResult) bool {
	m := make(map[string]cty.Value, len(result.Values))
	for k, v := range result.Values {
		if v != cty.NilVal {
			m[k] = v
		}
	}
	if len(m) == 0 {
		w.wctx.UpdateValue(cty.NilVal)
		return true
	}
	w.wctx.UpdateValue(cty.ObjectVal(m))
	return true
}
