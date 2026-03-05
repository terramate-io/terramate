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
	value   cty.Value
	cursor  int // 0 = first action, 1 = second action (if available)

	// SubFormRequest is populated when the widget signals WidgetNeedSubForm.
	SubFormRequest *SubFormRequest
}

func NewObjectWidget(wctx *WidgetContext, objType *typeschema.ObjectType) *ObjectWidget {
	return &ObjectWidget{
		wctx:    wctx,
		objType: objType,
		value:   cty.NilVal,
	}
}

func (w *ObjectWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

func (w *ObjectWidget) Prepare() {
	w.value = w.wctx.Value
	w.SubFormRequest = nil
	w.cursor = 0
}

func (w *ObjectWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	hasValue := w.value != cty.NilVal && !w.value.IsNull()
	maxCursor := 0
	if hasValue {
		maxCursor = 1
	}

	switch msg.Type {
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < maxCursor {
			w.cursor++
		}
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyEnter:
		if hasValue && w.cursor == 1 {
			w.value = cty.NilVal
			w.wctx.UpdateValue(cty.NilVal)
			w.cursor = 0
			return WidgetContinue, nil
		}
		req := &SubFormRequest{
			InputID:   w.wctx.Def.Name,
			InputDefs: objectAttrsToInputDefs(w.wctx.Def.ObjectAttributes),
			EditMode:  hasValue && w.cursor == 0,
		}
		if req.EditMode {
			req.Values = extractObjectAttrs(w.value)
		}
		w.SubFormRequest = req
		return WidgetNeedSubForm, nil
	}
	return WidgetContinue, nil
}

func (w *ObjectWidget) Render() []string {
	var lines []string

	hasValue := w.value != cty.NilVal && !w.value.IsNull()

	if hasValue && w.value.CanIterateElements() {
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
			if w.value.Type().HasAttribute(attr.Name) {
				v := w.value.GetAttr(attr.Name)
				line += " = " + valStyle.Render(FormatDisplayValue(v, attr.Type))
			} else {
				line += " " + dimStyle.Render("<not set>")
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	type action struct{ label string }
	var actions []action
	if hasValue {
		actions = []action{{"Edit values"}, {"Reset values"}}
	} else {
		actions = []action{{"Set values"}}
	}
	for i, a := range actions {
		if w.cursor == i {
			lines = append(lines, activeOptionStyle.Render("› "+a.label))
		} else {
			lines = append(lines, optionStyle.Render("  "+a.label))
		}
	}
	return lines
}

func (w *ObjectWidget) SetValue(val cty.Value) {
	w.value = val
}

func (w *ObjectWidget) FormatDisplay() string {
	if w.value == cty.NilVal || w.value.IsNull() {
		return "<not set>"
	}
	return FormatDisplayValue(w.value, w.objType)
}

func (w *ObjectWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

func (w *ObjectWidget) AcceptSubFormResult(result SubFormResult) bool {
	m := make(map[string]cty.Value, len(result.Values))
	for k, v := range result.Values {
		if v != cty.NilVal {
			m[k] = v
		}
	}
	if len(m) == 0 {
		w.value = cty.NilVal
		return true
	}
	w.value = cty.ObjectVal(m)
	if w.wctx != nil {
		w.wctx.UpdateValue(w.value)
	}
	return true
}
