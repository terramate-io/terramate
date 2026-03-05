// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/zclconf/go-cty/cty"
)

var errDuplicateKey = errors.E("Key already exists")

// ---------------------------------------------------------------------------
// MapWidget
// ---------------------------------------------------------------------------

type mapWidgetEntry struct {
	Key   string
	Value cty.Value
}

// InlineMapWidget manages a map of key-value pairs edited inline.
// This supports string and number values.
type InlineMapWidget struct {
	wctx        *WidgetContext
	valueType   typeschema.Type
	items       []mapWidgetEntry
	cursor      int  // 0..n-1 = items, n = "Add", n+1 = "Done"
	editIdx     int  // -1 = not editing; >= 0 = editing existing entry
	editKeyMode bool // when editing existing: true = editing key
	keyPhase    bool // when adding: true = entering key; false = entering value
	pendingKey  string
	textInput   textinput.Model

	validationErr error
	numberMode    bool
}

// NewInlineMapWidget creates a widget for editing a map of primitive key-value pairs inline.
func NewInlineMapWidget(wctx *WidgetContext, valueType typeschema.Type) *InlineMapWidget {
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

	return &InlineMapWidget{
		wctx:       wctx,
		valueType:  valueType,
		editIdx:    -1,
		textInput:  ti,
		numberMode: numberMode,
	}
}

// WidgetContext returns the widget's context.
func (w *InlineMapWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *InlineMapWidget) Prepare() {
	w.textInput.Reset()
	w.textInput.Focus()
	w.editIdx = -1
	w.keyPhase = true
	w.pendingKey = ""

	val := w.wctx.Value
	if val == cty.NilVal {
		val, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	}
	w.setMap(val)

	w.textInput.SetValue("")
	w.textInput.Placeholder = "Key"
	if w.wctx.Width > 12 {
		w.textInput.Width = w.wctx.Width - 12
	}
	w.cursor = len(w.items)
}

// Update handles keyboard input and returns the resulting signal.
func (w *InlineMapWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	n := len(w.items)

	// Editing an existing entry
	if w.editIdx >= 0 {
		if w.editKeyMode {
			switch msg.Type {
			case tea.KeyEnter:
				w.validationErr = nil
				key := w.textInput.Value()
				if key != "" {
					for i, entry := range w.items {
						if i != w.editIdx && entry.Key == key {
							w.validationErr = errDuplicateKey
							return WidgetContinue, nil
						}
					}
					w.items[w.editIdx] = mapWidgetEntry{Key: key, Value: w.items[w.editIdx].Value}
					w.editKeyMode = false
					w.textInput.SetValue(ctyToString(w.items[w.editIdx].Value))
					w.textInput.Placeholder = "Value"
				}
				return WidgetContinue, nil
			case tea.KeyEsc:
				w.editIdx = -1
				w.editKeyMode = false
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Blur()
				return WidgetContinue, nil
			default:
				w.validationErr = nil
				var cmd tea.Cmd
				w.textInput, cmd = w.textInput.Update(msg)
				return WidgetContinue, cmd
			}
		}
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
				w.items[w.editIdx] = mapWidgetEntry{Key: w.items[w.editIdx].Key, Value: editVal}
			}
			w.editIdx = -1
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Blur()
			return WidgetContinue, nil
		case tea.KeyBackspace, tea.KeyDelete:
			if w.textInput.Value() == "" {
				w.editKeyMode = true
				w.textInput.SetValue(w.items[w.editIdx].Key)
				w.textInput.Placeholder = "Key"
				return WidgetContinue, nil
			}
			var cmd tea.Cmd
			w.textInput, cmd = w.textInput.Update(msg)
			return WidgetContinue, cmd
		case tea.KeyEsc:
			w.editIdx = -1
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Blur()
			return WidgetContinue, nil
		default:
			w.validationErr = nil
			var cmd tea.Cmd
			w.textInput, cmd = w.textInput.Update(msg)
			return WidgetContinue, cmd
		}
	}

	// Cursor on "Done"
	if w.cursor == n+1 {
		switch msg.Type {
		case tea.KeyUp:
			w.cursor = n
			w.keyPhase = true
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Placeholder = "Key"
			w.textInput.Focus()
			if w.wctx.Width > 12 {
				w.textInput.Width = w.wctx.Width - 12
			}
			return WidgetContinue, nil
		case tea.KeyEnter:
			if !w.commitMap() {
				return WidgetContinue, nil
			}
			return WidgetConfirmed, nil
		case tea.KeyShiftTab, tea.KeyEsc:
			return WidgetBack, nil
		}
		return WidgetContinue, nil
	}

	// Cursor on "Add" action
	if w.cursor == n {
		if w.keyPhase {
			switch msg.Type {
			case tea.KeyUp:
				if n > 0 {
					w.keyPhase = false
					w.cursor = n - 1
					w.textInput.Blur()
				}
				return WidgetContinue, nil
			case tea.KeyDown:
				w.keyPhase = false
				w.cursor = n + 1
				w.textInput.Blur()
				return WidgetContinue, nil
			case tea.KeyEnter:
				w.validationErr = nil
				key := w.textInput.Value()
				if key == "" {
					return WidgetContinue, nil
				}
				for _, entry := range w.items {
					if entry.Key == key {
						w.validationErr = errDuplicateKey
						return WidgetContinue, nil
					}
				}
				w.pendingKey = key
				w.keyPhase = false
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Value"
				return WidgetContinue, nil
			case tea.KeyEsc:
				w.keyPhase = false
				w.pendingKey = ""
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Blur()
				return WidgetContinue, nil
			default:
				w.validationErr = nil
				var cmd tea.Cmd
				w.textInput, cmd = w.textInput.Update(msg)
				return WidgetContinue, cmd
			}
		} else if w.pendingKey != "" {
			switch msg.Type {
			case tea.KeyUp:
				w.pendingKey = ""
				w.keyPhase = false
				w.textInput.Blur()
				if n > 0 {
					w.cursor = n - 1
				}
				return WidgetContinue, nil
			case tea.KeyDown:
				w.pendingKey = ""
				w.keyPhase = false
				w.textInput.Blur()
				w.cursor = n + 1
				return WidgetContinue, nil
			case tea.KeyEnter:
				w.validationErr = nil
				textVal := w.textInput.Value()
				if textVal == "" {
					return WidgetContinue, nil
				}
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
				w.items = append(w.items, mapWidgetEntry{Key: w.pendingKey, Value: addVal})
				w.cursor = len(w.items)
				w.pendingKey = ""
				w.keyPhase = true
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Key"
				if w.wctx.Width > 12 {
					w.textInput.Width = w.wctx.Width - 12
				}
				return WidgetContinue, nil
			case tea.KeyEsc:
				w.pendingKey = ""
				w.keyPhase = true
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Key"
				return WidgetContinue, nil
			default:
				w.validationErr = nil
				var cmd tea.Cmd
				w.textInput, cmd = w.textInput.Update(msg)
				return WidgetContinue, cmd
			}
		}
	}

	// Cursor on an existing map entry
	switch msg.Type {
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n+1 {
			w.cursor++
			if w.cursor == n {
				w.keyPhase = true
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Key"
				w.textInput.Focus()
				if w.wctx.Width > 12 {
					w.textInput.Width = w.wctx.Width - 12
				}
			}
		}
	case tea.KeyEnter:
		w.editIdx = w.cursor
		w.textInput.Focus()
		w.textInput.SetValue(ctyToString(w.items[w.cursor].Value))
		w.textInput.Placeholder = "Value"
		if w.wctx.Width > 12 {
			w.textInput.Width = w.wctx.Width - 12
		}
	case tea.KeyDelete, tea.KeyBackspace:
		w.items = append(w.items[:w.cursor], w.items[w.cursor+1:]...)
		if w.cursor >= len(w.items) {
			w.cursor = len(w.items)
			w.keyPhase = true
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Placeholder = "Key"
			w.textInput.Focus()
			if w.wctx.Width > 12 {
				w.textInput.Width = w.wctx.Width - 12
			}
		}
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *InlineMapWidget) Render() []string {
	itemStyle := lipgloss.NewStyle().Foreground(colorText)
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	cursorItemStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)

	n := len(w.items)
	var lines []string

	maxKeyW := 0
	for _, entry := range w.items {
		if len(entry.Key) > maxKeyW {
			maxKeyW = len(entry.Key)
		}
	}

	for i, entry := range w.items {
		paddedKey := fmt.Sprintf("%-*s", maxKeyW, entry.Key)
		display := fmt.Sprintf("%s = %s", paddedKey, ctyToString(entry.Value))
		if w.editIdx == i {
			if w.editKeyMode {
				lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), w.textInput.View()))
			} else {
				lines = append(lines, fmt.Sprintf("  %s %s = %s", promptStyle.Render("›"), itemStyle.Render(paddedKey), w.textInput.View()))
			}
		} else if i == w.cursor {
			lines = append(lines, fmt.Sprintf("  %s %s  %s", promptStyle.Render("›"), cursorItemStyle.Render(display), hintStyle.Render("enter: edit · delete: remove")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("-"), itemStyle.Render(display)))
		}
	}

	if w.editIdx < 0 {
		if w.cursor == n {
			if w.pendingKey != "" {
				lines = append(lines, fmt.Sprintf("  %s %s = %s", promptStyle.Render("+"), itemStyle.Render(w.pendingKey), w.textInput.View()))
			} else {
				lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("+"), w.textInput.View()))
			}
		} else {
			lines = append(lines, dimStyle.Render("  + Add"))
		}
	}

	if w.validationErr != nil {
		lines = append(lines, validationStyle.Render(fmt.Sprintf("  %s", w.validationErr.Error())))
	}

	if w.cursor == n+1 {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), cursorItemStyle.Render("Done")))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render(" "), dimStyle.Render("Done")))
	}

	return lines
}

func (w *InlineMapWidget) commitMap() bool {
	w.validationErr = nil
	val := cty.EmptyObjectVal
	if len(w.items) > 0 {
		m := make(map[string]cty.Value, len(w.items))
		for _, entry := range w.items {
			m[entry.Key] = entry.Value
		}
		val = cty.ObjectVal(m)
	}
	w.wctx.UpdateValue(val)
	return true
}

func (w *InlineMapWidget) setMap(val cty.Value) {
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
		k, v := it.Element()
		w.items = append(w.items, mapWidgetEntry{Key: k.AsString(), Value: v})
	}
}

// FormatDisplay returns a compact summary of the current map value.
func (w *InlineMapWidget) FormatDisplay() string {
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
		k, v := it.Element()
		return fmt.Sprintf("%s = %s", k.AsString(), ctyToDisplayString(v))
	}
	return fmt.Sprintf("<%d entries>", n)
}

// ForwardMsg forwards a bubbletea message to the underlying text input.
func (w *InlineMapWidget) ForwardMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.textInput, cmd = w.textInput.Update(msg)
	return cmd
}

// AcceptSubFormResult is a no-op; inline map widgets do not use sub-forms.
func (w *InlineMapWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// SubFormMapWidget manages a map with complex values (objects, nested lists/maps,
// etc.) where each value is edited via a nested sub-form. Keys are entered inline.
type SubFormMapWidget struct {
	wctx       *WidgetContext
	valueType  typeschema.Type
	items      []subformMapEntry
	cursor     int  // 0..n-1 = items, n = "Add", n+1 = "Done"
	editIdx    int  // -1 = adding new; >= 0 = editing existing
	keyPhase   bool // true when entering a key on the "Add" row
	pendingKey string
	textInput  textinput.Model

	validationErr error

	SubFormRequest *SubFormRequest
}

type subformMapEntry struct {
	Key   string
	Value cty.Value
}

// NewSubFormMapWidget creates a widget for editing a map of complex values via nested sub-forms.
func NewSubFormMapWidget(wctx *WidgetContext, valueType typeschema.Type) InputWidget {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.Placeholder = "Key"
	if wctx.Width > 12 {
		ti.Width = wctx.Width - 12
	}

	return &SubFormMapWidget{
		wctx:      wctx,
		valueType: valueType,
		editIdx:   -1,
		textInput: ti,
	}
}

// WidgetContext returns the widget's context.
func (w *SubFormMapWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *SubFormMapWidget) Prepare() {
	w.SubFormRequest = nil
	w.editIdx = -1
	w.keyPhase = false
	w.pendingKey = ""
	w.validationErr = nil

	val := w.wctx.Value
	if val == cty.NilVal {
		val, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	}
	w.setMap(val)
	w.cursor = len(w.items)
	w.keyPhase = true
	w.textInput.Reset()
	w.textInput.SetValue("")
	w.textInput.Placeholder = "Key"
	w.textInput.Focus()
}

// Update handles keyboard input and returns the resulting signal.
func (w *SubFormMapWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	n := len(w.items)

	// Key entry phase on "Add" row
	if w.cursor == n && w.keyPhase {
		switch msg.Type {
		case tea.KeyUp:
			w.keyPhase = false
			w.textInput.Blur()
			if n > 0 {
				w.cursor = n - 1
			}
			return WidgetContinue, nil
		case tea.KeyDown:
			w.keyPhase = false
			w.textInput.Blur()
			w.cursor = n + 1
			return WidgetContinue, nil
		case tea.KeyEnter:
			key := w.textInput.Value()
			if key == "" {
				return WidgetContinue, nil
			}
			for _, entry := range w.items {
				if entry.Key == key {
					w.validationErr = errDuplicateKey
					return WidgetContinue, nil
				}
			}
			w.pendingKey = key
			w.keyPhase = false
			w.editIdx = -1
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Blur()
			w.SubFormRequest = w.buildSubFormRequest(false, cty.NilVal)
			w.SubFormRequest.Title = fmt.Sprintf("Add %s = ...", w.pendingKey)
			return WidgetNeedSubForm, nil
		case tea.KeyEsc:
			w.keyPhase = false
			w.pendingKey = ""
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Blur()
			return WidgetContinue, nil
		case tea.KeyShiftTab:
			return WidgetBack, nil
		default:
			w.validationErr = nil
			var cmd tea.Cmd
			w.textInput, cmd = w.textInput.Update(msg)
			return WidgetContinue, cmd
		}
	}

	// "Done" row
	if w.cursor == n+1 {
		switch msg.Type {
		case tea.KeyUp:
			w.cursor = n
			w.keyPhase = true
			w.textInput.Reset()
			w.textInput.SetValue("")
			w.textInput.Placeholder = "Key"
			w.textInput.Focus()
			return WidgetContinue, nil
		case tea.KeyEnter:
			w.commitMap()
			return WidgetConfirmed, nil
		case tea.KeyShiftTab, tea.KeyEsc:
			return WidgetBack, nil
		}
		return WidgetContinue, nil
	}

	// "Add" row but NOT in key phase (shouldn't normally happen, but handle gracefully)
	if w.cursor == n && !w.keyPhase {
		w.keyPhase = true
		w.textInput.Reset()
		w.textInput.SetValue("")
		w.textInput.Placeholder = "Key"
		w.textInput.Focus()
		return WidgetContinue, nil
	}

	// Existing entry
	switch msg.Type {
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n+1 {
			w.cursor++
			if w.cursor == n {
				w.keyPhase = true
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Key"
				w.textInput.Focus()
			}
		}
	case tea.KeyEnter:
		w.editIdx = w.cursor
		w.SubFormRequest = w.buildSubFormRequest(true, w.items[w.cursor].Value)
		w.SubFormRequest.Title = fmt.Sprintf("Edit %s = ...", w.items[w.cursor].Key)
		return WidgetNeedSubForm, nil
	case tea.KeyDelete, tea.KeyBackspace:
		if w.cursor < n && n > 0 {
			w.items = append(w.items[:w.cursor], w.items[w.cursor+1:]...)
			if w.cursor >= len(w.items) {
				w.cursor = len(w.items)
				w.keyPhase = true
				w.textInput.Reset()
				w.textInput.SetValue("")
				w.textInput.Placeholder = "Key"
				w.textInput.Focus()
			}
		}
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *SubFormMapWidget) Render() []string {
	itemStyle := lipgloss.NewStyle().Foreground(colorText)
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	cursorItemStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)

	n := len(w.items)
	var lines []string

	maxKeyW := 0
	for _, entry := range w.items {
		if len(entry.Key) > maxKeyW {
			maxKeyW = len(entry.Key)
		}
	}

	for i, entry := range w.items {
		paddedKey := fmt.Sprintf("%-*s", maxKeyW, entry.Key)
		summary := FormatDisplayValue(entry.Value, w.valueType)
		display := fmt.Sprintf("%s = %s", paddedKey, summary)
		if w.cursor == i {
			lines = append(lines, fmt.Sprintf("  %s %s  %s", promptStyle.Render("›"), cursorItemStyle.Render(display), hintStyle.Render("enter: edit · delete: remove")))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render("-"), itemStyle.Render(display)))
		}
	}

	if w.cursor == n {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("+"), w.textInput.View()))
	} else {
		lines = append(lines, dimStyle.Render("  + Add"))
	}

	if w.validationErr != nil {
		lines = append(lines, validationStyle.Render(fmt.Sprintf("  %s", w.validationErr.Error())))
	}

	if w.cursor == n+1 {
		lines = append(lines, fmt.Sprintf("  %s %s", promptStyle.Render("›"), cursorItemStyle.Render("Done")))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", dimStyle.Render(" "), dimStyle.Render("Done")))
	}

	return lines
}

// FormatDisplay returns a compact summary of the current map value.
func (w *SubFormMapWidget) FormatDisplay() string {
	n := len(w.items)
	if n == 0 {
		return "<empty>"
	}
	if n == 1 {
		return fmt.Sprintf("%s = %s", w.items[0].Key, FormatDisplayValue(w.items[0].Value, w.valueType))
	}
	return fmt.Sprintf("<%d entries>", n)
}

// ForwardMsg forwards a bubbletea message to the underlying text input.
func (w *SubFormMapWidget) ForwardMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.textInput, cmd = w.textInput.Update(msg)
	return cmd
}

// AcceptSubFormResult integrates a completed sub-form result into the map.
func (w *SubFormMapWidget) AcceptSubFormResult(result SubFormResult) bool {
	val := subFormResultToValue(w.valueType, result.Values)
	if val == cty.NilVal {
		return false
	}
	if w.editIdx >= 0 && w.editIdx < len(w.items) {
		w.items[w.editIdx].Value = val
	} else {
		w.items = append(w.items, subformMapEntry{Key: w.pendingKey, Value: val})
		w.cursor = len(w.items)
	}
	w.pendingKey = ""
	w.editIdx = -1
	w.keyPhase = true
	w.textInput.Reset()
	w.textInput.SetValue("")
	w.textInput.Placeholder = "Key"
	w.textInput.Focus()
	return false
}

func (w *SubFormMapWidget) setMap(val cty.Value) {
	w.items = nil
	if val == cty.NilVal || val.IsNull() || !val.CanIterateElements() {
		return
	}
	it := val.ElementIterator()
	for it.Next() {
		k, v := it.Element()
		w.items = append(w.items, subformMapEntry{Key: k.AsString(), Value: v})
	}
}

func (w *SubFormMapWidget) commitMap() {
	val := cty.EmptyObjectVal
	if len(w.items) > 0 {
		m := make(map[string]cty.Value, len(w.items))
		for _, entry := range w.items {
			m[entry.Key] = entry.Value
		}
		val = cty.ObjectVal(m)
	}
	w.wctx.UpdateValue(val)
}

func (w *SubFormMapWidget) buildSubFormRequest(editMode bool, existing cty.Value) *SubFormRequest {
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
