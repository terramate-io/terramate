// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/zclconf/go-cty/cty"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextWidget wraps a textinput.Model for single-line text inputs.
// This can be either a string or a number.
type TextWidget struct {
	wctx          *WidgetContext
	valueType     typeschema.Type
	defaultValue  cty.Value
	textInput     textinput.Model
	validationErr error
	numberMode    bool
}

// NewTextWidget creates a single-line text input widget for strings or numbers.
func NewTextWidget(wctx *WidgetContext, valueType typeschema.Type) *TextWidget {
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

	return &TextWidget{
		wctx:       wctx,
		valueType:  valueType,
		textInput:  ti,
		numberMode: numberMode,
	}
}

// WidgetContext returns the widget's context.
func (w *TextWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *TextWidget) Prepare() {
	w.textInput.Reset()
	w.textInput.Focus()

	w.defaultValue, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)

	w.textInput.SetValue(ctyToString(w.wctx.Value))
	w.textInput.Placeholder = ctyToDisplayString(w.defaultValue)
}

// Update handles keyboard input and returns the resulting signal.
func (w *TextWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		w.validationErr = nil

		var newVal cty.Value
		textVal := w.textInput.Value()

		if textVal != "" {
			if w.numberMode {
				v, err := cty.ParseNumberVal(textVal)
				if err != nil {
					w.validationErr = errors.E("This field requires a number.")
					return WidgetContinue, nil
				}
				newVal = v
			} else {
				newVal = cty.StringVal(textVal)
			}
		} else {
			if w.defaultValue == cty.NilVal {
				w.validationErr = errors.E("This field is required.")
				return WidgetContinue, nil
			}
			newVal = w.defaultValue
		}

		w.wctx.UpdateValue(newVal)
		return WidgetConfirmed, nil

	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil

	default:
		w.validationErr = nil

		var cmd tea.Cmd
		w.textInput, cmd = w.textInput.Update(msg)
		return WidgetContinue, cmd
	}
}

// Render returns the rendered display lines for the widget.
func (w *TextWidget) Render() []string {
	lines := []string{fmt.Sprintf("  %s %s", promptStyle.Render("›"), w.textInput.View())}
	if w.validationErr != nil {
		lines = append(lines, "", validationStyle.Render(fmt.Sprintf("  %s", w.validationErr.Error())))
	}
	return lines
}

// FormatDisplay returns a display string for the current value.
func (w *TextWidget) FormatDisplay() string {
	return ctyToDisplayString(w.wctx.Value)
}

// ForwardMsg forwards a bubbletea message to the underlying text input.
func (w *TextWidget) ForwardMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.textInput, cmd = w.textInput.Update(msg)
	return cmd
}

// AcceptSubFormResult is a no-op; text widgets do not use sub-forms.
func (w *TextWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// BoolWidget provides a Yes/No toggle.
type BoolWidget struct {
	wctx   *WidgetContext
	cursor bool
}

// NewBoolWidget creates a Yes/No toggle widget.
func NewBoolWidget(wctx *WidgetContext) *BoolWidget {
	return &BoolWidget{
		wctx: wctx,
	}
}

// WidgetContext returns the widget's context.
func (w *BoolWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *BoolWidget) Prepare() {
	val := w.wctx.Value
	if val != cty.NilVal {
		w.cursor = val.True()
		return
	}

	defaultValue, _ := w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	if defaultValue != cty.NilVal {
		w.cursor = defaultValue.True()
		return
	}

	w.cursor = false
}

// Update handles keyboard input and returns the resulting signal.
func (w *BoolWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyLeft:
		w.cursor = true
	case tea.KeyDown, tea.KeyRight:
		w.cursor = false
	case tea.KeyEnter:
		w.wctx.UpdateValue(cty.BoolVal(w.cursor))
		return WidgetConfirmed, nil
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *BoolWidget) Render() []string {
	var yesBtn, noBtn string
	if w.cursor {
		yesBtn = boolActiveStyle.Render("Yes")
		noBtn = boolInactiveStyle.Render("No")
	} else {
		yesBtn = boolInactiveStyle.Render("Yes")
		noBtn = boolActiveStyle.Render("No")
	}
	toggle := lipgloss.JoinHorizontal(lipgloss.Top, "  ", yesBtn, "  ", noBtn)
	return []string{toggle}
}

// FormatDisplay returns a display string for the current boolean value.
func (w *BoolWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val != cty.NilVal {
		if val.True() {
			return "Yes"
		}
		return "No"
	}
	return ""
}

// ForwardMsg is a no-op; bool widgets have no underlying input component.
func (w *BoolWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult is a no-op; bool widgets do not use sub-forms.
func (w *BoolWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// MultilineWidget wraps a textarea.Model for multi-line text inputs.
type MultilineWidget struct {
	wctx          *WidgetContext
	defaultValue  cty.Value
	textArea      textarea.Model
	validationErr error
}

// NewMultilineWidget creates a multi-line text area widget.
func NewMultilineWidget(wctx *WidgetContext) *MultilineWidget {
	ta := textarea.New()
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetWidth(wctx.Width - 16)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false

	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorTextSubtle)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorText)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle()
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle()
	ta.BlurredStyle = ta.FocusedStyle
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(colorTextMuted)

	return &MultilineWidget{
		wctx:     wctx,
		textArea: ta,
	}
}

// WidgetContext returns the widget's context.
func (w *MultilineWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *MultilineWidget) Prepare() {
	w.textArea.Reset()
	w.textArea.Focus()

	w.defaultValue, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)

	w.textArea.SetValue(ctyToString(w.wctx.Value))
	w.textArea.Placeholder = ctyToDisplayString(w.defaultValue)
}

// Update handles keyboard input and returns the resulting signal.
func (w *MultilineWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlS:
		w.validationErr = nil

		var newVal cty.Value
		textVal := w.textArea.Value()

		if textVal != "" {
			newVal = cty.StringVal(textVal)
		} else {
			if w.defaultValue == cty.NilVal {
				w.validationErr = errors.E("This field is required.")
				return WidgetContinue, nil
			}
			newVal = w.defaultValue
		}

		w.wctx.UpdateValue(newVal)
		w.textArea.Blur()
		return WidgetConfirmed, nil
	case tea.KeyEsc, tea.KeyShiftTab:
		w.textArea.Blur()
		return WidgetBack, nil
	default:
		w.validationErr = nil

		var cmd tea.Cmd
		w.textArea, cmd = w.textArea.Update(msg)
		return WidgetContinue, cmd
	}
}

// Render returns the rendered display lines for the widget.
func (w *MultilineWidget) Render() []string {
	taView := w.textArea.View()
	totalLines := w.textArea.LineCount()
	visibleLines := w.textArea.Height()

	scrollbarGutter := 2
	boxWidth := w.wctx.Width - 6 - 4 - scrollbarGutter
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(boxWidth)

	bordered := boxStyle.Render(taView)

	if totalLines > visibleLines {
		sb := renderTextareaScrollbar(lipgloss.Height(bordered), totalLines, w.textArea.Line())
		bordered = lipgloss.JoinHorizontal(lipgloss.Top, bordered, " ", sb)
	}

	bLines := strings.Split(bordered, "\n")
	var result []string
	for _, line := range bLines {
		result = append(result, "  "+line)
	}
	if w.validationErr != nil {
		result = append(result, "", validationStyle.Render(fmt.Sprintf("  %s", w.validationErr.Error())))
	}
	result = append(result, "")
	result = append(result, defaultStyle.Render("  ctrl+s: confirm • esc: cancel"))
	return result
}

// renderTextareaScrollbar builds a vertical scrollbar track for the multiline textarea.
// Position is derived from the cursor line since the textarea's internal viewport offset is private.
func renderTextareaScrollbar(trackHeight, totalLines, cursorLine int) string {
	if trackHeight < 1 {
		trackHeight = 1
	}

	thumbSize := (trackHeight * trackHeight) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	var scrollFraction float64
	if totalLines > 1 {
		scrollFraction = float64(cursorLine) / float64(totalLines-1)
	}
	thumbPos := int(scrollFraction * float64(trackHeight-thumbSize))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > trackHeight {
		thumbPos = trackHeight - thumbSize
	}

	trackStyle := lipgloss.NewStyle().Foreground(colorScrollTrack)
	thumbStyle := lipgloss.NewStyle().Foreground(colorScrollThumb)

	var sb strings.Builder
	for i := range trackHeight {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(thumbStyle.Render("┃"))
		} else {
			sb.WriteString(trackStyle.Render("│"))
		}
	}
	return sb.String()
}

// FormatDisplay returns a truncated display string for the current value.
func (w *MultilineWidget) FormatDisplay() string {
	s := ctyToDisplayString(w.wctx.Value)
	truncated := false
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
		truncated = true
	}
	if len(s) > 20 {
		s = s[:20]
		truncated = true
	}
	if truncated {
		s += "..."
	}
	return s
}

// ForwardMsg forwards a bubbletea message to the underlying text area.
func (w *MultilineWidget) ForwardMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.textArea, cmd = w.textArea.Update(msg)
	return cmd
}

// AcceptSubFormResult is a no-op; multiline widgets do not use sub-forms.
func (w *MultilineWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ctyToString(v cty.Value) string {
	if v == cty.NilVal || v.IsNull() {
		return ""
	}
	switch v.Type() {
	case cty.String:
		return v.AsString()
	case cty.Number:
		bf := v.AsBigFloat()
		return bf.Text('f', -1)
	default:
		panic("unsupported value type " + v.GoString())
	}
}

func ctyToDisplayString(v cty.Value) string {
	if v == cty.NilVal || v.IsNull() {
		return ""
	}
	switch v.Type() {
	case cty.String:
		return v.AsString()
	case cty.Number:
		bf := v.AsBigFloat()
		return bf.Text('f', -1)
	case cty.Bool:
		if v.True() {
			return "true"
		}
		return "false"
	default:
		if v.CanIterateElements() {
			return fmt.Sprintf("<%d items>", v.LengthInt())
		}
		return v.GoString()
	}
}
