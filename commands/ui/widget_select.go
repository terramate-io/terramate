// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zclconf/go-cty/cty"
)

// SelectWidget provides a cursor-based single-select option list.
type SelectWidget struct {
	wctx    *WidgetContext
	options []InputOption
	cursor  int
	value   cty.Value
}

// NewSelectWidget creates a single-select option list widget.
func NewSelectWidget(wctx *WidgetContext) *SelectWidget {
	return &SelectWidget{
		wctx:  wctx,
		value: cty.NilVal,
	}
}

// WidgetContext returns the widget's context.
func (w *SelectWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *SelectWidget) Prepare() {
	if w.wctx.Value != cty.NilVal {
		w.setValue(w.wctx.Value)
	}
	w.options = resolveInputOptions(w.wctx)
	w.cursor = 0

	if w.value != cty.NilVal && !w.value.IsNull() {
		for i, opt := range w.options {
			if optValEquals(opt.Value, w.value) {
				w.cursor = i
				break
			}
		}
	} else {
		defaultValue, _ := w.wctx.Def.EvalDefault(w.wctx.Schemactx)
		if defaultValue != cty.NilVal && !defaultValue.IsNull() {
			for i, opt := range w.options {
				if optValEquals(opt.Value, defaultValue) {
					w.cursor = i
					break
				}
			}
		}
	}
}

// Update handles keyboard input and returns the resulting signal.
func (w *SelectWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < len(w.options)-1 {
			w.cursor++
		}
	case tea.KeyEnter:
		if len(w.options) > 0 {
			w.value = w.options[w.cursor].Value
			w.wctx.UpdateValue(w.value)
		}
		return WidgetConfirmed, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *SelectWidget) Render() []string {
	return renderOptionsList(w.options, w.cursor, nil)
}

func (w *SelectWidget) setValue(val cty.Value) {
	w.value = val
	if val == cty.NilVal || val.IsNull() {
		return
	}
	for i, opt := range w.options {
		if optValEquals(opt.Value, val) {
			w.cursor = i
			return
		}
	}
}

// FormatDisplay returns a display string for the currently selected option.
func (w *SelectWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() {
		return ""
	}
	for _, opt := range w.options {
		if optValEquals(opt.Value, val) {
			return opt.Label
		}
	}
	return ctyToDisplayString(val)
}

// ForwardMsg is a no-op; select widgets have no underlying input component.
func (w *SelectWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult is a no-op; select widgets do not use sub-forms.
func (w *SelectWidget) AcceptSubFormResult(SubFormResult) bool {
	return true
}

// MultiSelectWidget provides a cursor-based multi-select with checkboxes.
type MultiSelectWidget struct {
	wctx          *WidgetContext
	options       []InputOption
	selected      map[int]bool
	cursor        int
	value         cty.Value
	validationErr error
}

// NewMultiSelectWidget creates a multi-select option list widget with checkboxes.
func NewMultiSelectWidget(wctx *WidgetContext) *MultiSelectWidget {
	return &MultiSelectWidget{
		wctx:     wctx,
		selected: map[int]bool{},
		value:    cty.NilVal,
	}
}

// WidgetContext returns the widget's context.
func (w *MultiSelectWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *MultiSelectWidget) Prepare() {
	if w.wctx.Value != cty.NilVal {
		w.setValue(w.wctx.Value)
	}
	w.options = resolveInputOptions(w.wctx)
	w.cursor = 0

	if w.value != cty.NilVal && !w.value.IsNull() && w.value.CanIterateElements() {
		w.selected = map[int]bool{}
		it := w.value.ElementIterator()
		for it.Next() {
			_, elem := it.Element()
			for i, opt := range w.options {
				if optValEquals(opt.Value, elem) {
					w.selected[i] = true
					break
				}
			}
		}
	} else {
		w.selected = map[int]bool{}
		defaultValue, _ := w.wctx.Def.EvalDefault(w.wctx.Schemactx)
		if defaultValue != cty.NilVal && !defaultValue.IsNull() && defaultValue.CanIterateElements() {
			it := defaultValue.ElementIterator()
			for it.Next() {
				_, elem := it.Element()
				for i, opt := range w.options {
					if optValEquals(opt.Value, elem) {
						w.selected[i] = true
						break
					}
				}
			}
		}
	}
}

// Update handles keyboard input and returns the resulting signal.
func (w *MultiSelectWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < len(w.options)-1 {
			w.cursor++
		}
	case tea.KeySpace:
		w.selected[w.cursor] = !w.selected[w.cursor]
	case tea.KeyEnter:
		w.validationErr = nil
		var vals []cty.Value
		for i, opt := range w.options {
			if w.selected[i] {
				vals = append(vals, opt.Value)
			}
		}
		val := cty.NilVal
		if len(vals) > 0 {
			val = cty.TupleVal(vals)
		}
		w.wctx.UpdateValue(val)
		w.value = val
		return WidgetConfirmed, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *MultiSelectWidget) Render() []string {
	lines := renderOptionsList(w.options, w.cursor, w.selected)
	if w.validationErr != nil {
		errStyle := validationStyle.PaddingLeft(2).Width(w.wctx.Width)
		lines = append(lines, "", errStyle.Render(w.validationErr.Error()))
	}
	return lines
}

func (w *MultiSelectWidget) setValue(val cty.Value) {
	w.value = val
	w.selected = map[int]bool{}
	if val == cty.NilVal || val.IsNull() || !val.CanIterateElements() {
		return
	}
	it := val.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		for i, opt := range w.options {
			if optValEquals(opt.Value, elem) {
				w.selected[i] = true
				break
			}
		}
	}
}

// FormatDisplay returns a comma-separated display string of the selected options.
func (w *MultiSelectWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() {
		return "<none>"
	}
	if len(w.options) > 0 {
		var labels []string
		for i, opt := range w.options {
			if w.selected[i] {
				labels = append(labels, opt.Label)
			}
		}
		if len(labels) == 0 {
			return "<none>"
		}
		return strings.Join(labels, ", ")
	}
	return ctyToDisplayString(val)
}

// ForwardMsg is a no-op; multi-select widgets have no underlying input component.
func (w *MultiSelectWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult is a no-op; multi-select widgets do not use sub-forms.
func (w *MultiSelectWidget) AcceptSubFormResult(SubFormResult) bool {
	return true
}

// BundleRefWidget lets the user pick an existing created bundle or create a new one.
type BundleRefWidget struct {
	wctx            *WidgetContext
	classID         string
	cursor          int
	value           cty.Value
	PendingRefClass string
}

// NewBundleRefWidget creates a widget for selecting or creating a bundle reference.
func NewBundleRefWidget(wctx *WidgetContext, classID string) *BundleRefWidget {
	return &BundleRefWidget{
		wctx:    wctx,
		classID: classID,
		value:   cty.NilVal,
	}
}

// WidgetContext returns the widget's context.
func (w *BundleRefWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *BundleRefWidget) Prepare() {
	w.value = w.wctx.Value
	w.cursor = 0
}

// Update handles keyboard input and returns the resulting signal.
func (w *BundleRefWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	matching := w.wctx.Registry.MatchingBundleOptions(w.classID, w.wctx.Env)
	n := len(matching) + 1 // +1 for "Add new" option

	switch msg.Type {
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n-1 {
			w.cursor++
		}
	case tea.KeyEnter:
		if w.cursor < len(matching) {
			w.value = cty.StringVal(matching[w.cursor].Alias)
			w.wctx.UpdateValue(w.value)
			return WidgetConfirmed, nil
		}
		w.PendingRefClass = w.classID
		return WidgetNeedSubForm, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *BundleRefWidget) Render() []string {
	matching := w.wctx.Registry.MatchingBundleOptions(w.classID, w.wctx.Env)
	var lines []string
	for i, b := range matching {
		label := b.Alias
		if b.EnvID != "" {
			label += " [" + b.EnvID + "]"
		}
		if i == w.cursor {
			lines = append(lines, activeOptionStyle.Render("› "+label))
		} else {
			lines = append(lines, optionStyle.Render("  "+label))
		}
	}
	if w.cursor == len(matching) {
		lines = append(lines, activeOptionStyle.Render("› + Add new"))
	} else {
		lines = append(lines, dimOptionStyle.Render("  + Add new"))
	}
	return lines
}

// Reload syncs the widget's internal state from the WidgetContext value.
func (w *BundleRefWidget) Reload() {
	w.value = w.wctx.Value
}

// FormatDisplay returns a display string for the selected bundle reference.
func (w *BundleRefWidget) FormatDisplay() string {
	// FormatDisplay may be called before Prepare(), so check wctx.Value too.
	val := w.value
	if val == cty.NilVal {
		val = w.wctx.Value
	}
	if val == cty.NilVal || val.IsNull() {
		return "<not set>"
	}
	// The value can be a string (alias), a resolved bundle object, or a
	// cty.DynamicVal placeholder (unresolved null).
	if !val.IsKnown() {
		return "<not set>"
	}
	var alias string
	switch {
	case val.Type() == cty.String:
		alias = val.AsString()
	case val.Type().IsObjectType() && val.Type().HasAttribute("alias"):
		a := val.GetAttr("alias")
		if a.IsKnown() && a.Type() == cty.String {
			alias = a.AsString()
		} else {
			return ctyToDisplayString(val)
		}
	default:
		return ctyToDisplayString(val)
	}
	for _, opt := range w.wctx.Registry.MatchingBundleOptions(w.classID, w.wctx.Env) {
		if opt.Alias == alias {
			return opt.Name
		}
	}
	if len(alias) > 8 {
		return alias[:8] + "..."
	}
	return alias
}

// ForwardMsg is a no-op; bundle-ref widgets have no underlying input component.
func (w *BundleRefWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult is a no-op; bundle-ref widgets do not use sub-forms.
func (w *BundleRefWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// resolveInputOptions builds the option list from a definition's options.
func resolveInputOptions(ctx *WidgetContext) []InputOption {
	if !ctx.Def.HasPromptOptions() {
		return nil
	}

	namedVals, err := ctx.Def.EvalPromptOptions(ctx.Schemactx)
	if err != nil || namedVals == nil {
		return nil
	}

	opts := make([]InputOption, len(namedVals))
	for i, nv := range namedVals {
		opts[i] = InputOption{
			Label: nv.Name,
			Value: nv.Value,
		}
	}
	return opts
}

// renderOptionsList renders a cursor-based option list.
// If selected is non-nil, checkboxes are rendered (multiselect mode).
func renderOptionsList(options []InputOption, cursor int, selected map[int]bool) []string {
	var lines []string
	for i, opt := range options {
		var prefix string
		if selected != nil {
			if selected[i] {
				prefix = checkboxOn.Render("[✓]")
			} else {
				prefix = checkboxOff.Render("[ ]")
			}
			prefix += " "
		}

		if i == cursor {
			lines = append(lines, activeOptionStyle.Render(fmt.Sprintf("› %s%s", prefix, opt.Label)))
		} else {
			lines = append(lines, optionStyle.Render(fmt.Sprintf("  %s%s", prefix, opt.Label)))
		}
	}
	return lines
}

// optValEquals compares two cty.Values for equality, handling NilVal.
func optValEquals(a, b cty.Value) bool {
	if a == cty.NilVal || b == cty.NilVal {
		return a == b
	}
	if a.IsNull() || b.IsNull() {
		return a.IsNull() && b.IsNull()
	}
	eq := a.Equals(b)
	return eq.Type() == cty.Bool && eq.True()
}
