// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/typeschema"
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
	if len(w.options) > 0 && val.CanIterateElements() {
		var labels []string
		for _, opt := range w.options {
			it := val.ElementIterator()
			for it.Next() {
				_, elem := it.Element()
				if optValEquals(opt.Value, elem) {
					labels = append(labels, opt.Label)
					break
				}
			}
		}
		if len(labels) == 0 {
			return "<none>"
		}
		return strings.Join(labels, ", ")
	}
	return ctyToDisplayString(val)
}

// HelpHints returns the key hints shown in the bottom help line while the
// multi-select is the active input.
func (w *MultiSelectWidget) HelpHints() string {
	return "enter: confirm • space: toggle"
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
	matching := MatchingBundleOptions(w.wctx.Registry, w.classID, w.wctx.Env)
	n := len(matching) + 1 // +1 for the "Add new" row

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
	matching := MatchingBundleOptions(w.wctx.Registry, w.classID, w.wctx.Env)
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
		lines = append(lines, activeOptionStyle.Render("+ Add new"))
	} else {
		lines = append(lines, dimOptionStyle.Render("  Add new"))
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
	for _, opt := range MatchingBundleOptions(w.wctx.Registry, w.classID, w.wctx.Env) {
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

// bundleRefAcceptor is implemented by widgets that can take over the alias of a
// bundle the user created inline through the "Add new" row.
type bundleRefAcceptor interface {
	AcceptCreatedBundleRef(alias string) (done bool)
}

// BundleRefListWidget lets the user pick any number of bundles of one class.
type BundleRefListWidget struct {
	wctx     *WidgetContext
	classID  string
	selected []string // Aliases, in selection order.
	cursor   int      // 0..n-1 = bundles, n = "Add new"

	PendingRefClass string
}

// NewBundleRefListWidget creates a widget for selecting multiple bundle references.
func NewBundleRefListWidget(wctx *WidgetContext, classID string) *BundleRefListWidget {
	return &BundleRefListWidget{
		wctx:    wctx,
		classID: classID,
	}
}

// WidgetContext returns the widget's context.
func (w *BundleRefListWidget) WidgetContext() *WidgetContext {
	return w.wctx
}

// Prepare initializes the widget for a new editing session.
func (w *BundleRefListWidget) Prepare() {
	w.PendingRefClass = ""
	val := w.wctx.Value
	if val == cty.NilVal {
		val, _ = w.wctx.Def.EvalDefault(w.wctx.Schemactx)
	}
	w.setSelection(val)
	w.cursor = 0
}

// bundleRefRow is one selectable bundle in the widget. Rows come from the registry, plus
// any selected alias the registry does not know about - a bundle created during
// this session that a stale registry has not picked up yet.
type bundleRefRow struct {
	alias string
	label string
}

func (w *BundleRefListWidget) rows() []bundleRefRow {
	var rows []bundleRefRow
	known := map[string]bool{}
	for _, opt := range MatchingBundleOptions(w.wctx.Registry, w.classID, w.wctx.Env) {
		label := opt.Name
		if label == "" {
			label = opt.Alias
		}
		if opt.EnvID != "" {
			label += " [" + opt.EnvID + "]"
		}
		rows = append(rows, bundleRefRow{alias: opt.Alias, label: label})
		known[opt.Alias] = true
	}
	for _, alias := range w.selected {
		if !known[alias] {
			rows = append(rows, bundleRefRow{alias: alias, label: alias})
			known[alias] = true
		}
	}
	return rows
}

// Update handles keyboard input and returns the resulting signal.
func (w *BundleRefListWidget) Update(msg tea.KeyMsg) (WidgetSignal, tea.Cmd) {
	rows := w.rows()
	n := len(rows)

	switch msg.Type {
	case tea.KeyShiftTab, tea.KeyEsc:
		return WidgetBack, nil
	case tea.KeyUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case tea.KeyDown:
		if w.cursor < n {
			w.cursor++
		}
	case tea.KeySpace:
		if w.cursor < n {
			w.toggle(rows[w.cursor].alias)
		}
	case tea.KeyEnter:
		if w.cursor == n {
			w.PendingRefClass = w.classID
			return WidgetNeedSubForm, nil
		}
		w.commit()
		return WidgetConfirmed, nil
	}
	return WidgetContinue, nil
}

// Render returns the rendered display lines for the widget.
func (w *BundleRefListWidget) Render() []string {
	rows := w.rows()
	n := len(rows)

	var lines []string
	for i, row := range rows {
		prefix := checkboxOff.Render("[ ]")
		if w.isSelected(row.alias) {
			prefix = checkboxOn.Render("[✓]")
		}
		if i == w.cursor {
			lines = append(lines, activeOptionStyle.Render(fmt.Sprintf("› %s %s", prefix, row.label)))
		} else {
			lines = append(lines, optionStyle.Render(fmt.Sprintf("  %s %s", prefix, row.label)))
		}
	}

	// The add row carries its own marker: the "+" sits in the cursor column
	// instead of the "›" the bundle rows use.
	if w.cursor == n {
		lines = append(lines, activeOptionStyle.Render("+ Add new"))
	} else {
		lines = append(lines, dimOptionStyle.Render("  Add new"))
	}
	return lines
}

// HelpHints returns the key hints shown in the bottom help line while the
// bundle multi-select is the active input. The "Add new" row speaks for itself,
// so it drops the hints rather than repeating what its label already says - and
// the selection hints would be wrong there anyway.
func (w *BundleRefListWidget) HelpHints() string {
	if w.cursor == len(w.rows()) {
		return ""
	}
	return "enter: confirm • space: toggle"
}

// Reload syncs the widget's internal state from the WidgetContext value.
func (w *BundleRefListWidget) Reload() {
	w.setSelection(w.wctx.Value)
}

// FormatDisplay returns a comma-separated summary of the selected bundles.
func (w *BundleRefListWidget) FormatDisplay() string {
	val := w.wctx.Value
	if val == cty.NilVal || val.IsNull() || !val.IsKnown() || !val.CanIterateElements() {
		return "<none>"
	}
	if val.LengthInt() == 0 {
		return "<none>"
	}
	return FormatDisplayValue(val, &typeschema.ListType{
		ValueType: &typeschema.BundleType{ClassID: w.classID},
	})
}

// ForwardMsg is a no-op; bundle-ref lists have no underlying input component.
func (w *BundleRefListWidget) ForwardMsg(tea.Msg) tea.Cmd {
	return nil
}

// AcceptSubFormResult is a no-op; bundle-ref lists do not use sub-forms.
func (w *BundleRefListWidget) AcceptSubFormResult(SubFormResult) bool { return true }

// AcceptCreatedBundleRef adds the newly created bundle to the selection. The
// input stays active so that more references can be added.
func (w *BundleRefListWidget) AcceptCreatedBundleRef(alias string) bool {
	w.PendingRefClass = ""
	if !w.isSelected(alias) {
		w.selected = append(w.selected, alias)
	}
	w.commit()
	// Park the cursor on "Add new" again, which is where the user left off.
	w.cursor = len(w.rows())
	return false
}

func (w *BundleRefListWidget) isSelected(alias string) bool {
	return slices.Contains(w.selected, alias)
}

func (w *BundleRefListWidget) toggle(alias string) {
	if i := slices.Index(w.selected, alias); i >= 0 {
		w.selected = slices.Delete(w.selected, i, i+1)
		return
	}
	w.selected = append(w.selected, alias)
}

// setSelection reads the selection from an input value. Values loaded from disk
// hold resolved bundle objects, so each element is reduced to its alias.
func (w *BundleRefListWidget) setSelection(val cty.Value) {
	w.selected = nil
	if val == cty.NilVal || val.IsNull() || !val.IsKnown() || !val.CanIterateElements() {
		return
	}
	for it := val.ElementIterator(); it.Next(); {
		_, elem := it.Element()
		if alias, ok := bundleRefAlias(elem); ok && !w.isSelected(alias) {
			w.selected = append(w.selected, alias)
		}
	}
}

func (w *BundleRefListWidget) commit() {
	val := cty.EmptyTupleVal
	if len(w.selected) > 0 {
		elems := make([]cty.Value, len(w.selected))
		for i, alias := range w.selected {
			elems[i] = cty.StringVal(alias)
		}
		val = cty.TupleVal(elems)
	}
	w.wctx.UpdateValue(val)
}

// bundleRefAlias extracts the alias from a bundle reference value, which is
// either a key string or a resolved bundle object.
func bundleRefAlias(val cty.Value) (string, bool) {
	if val == cty.NilVal || val.IsNull() || !val.IsKnown() {
		return "", false
	}
	switch {
	case val.Type() == cty.String:
		return val.AsString(), true
	case val.Type().IsObjectType() && val.Type().HasAttribute("alias"):
		alias := val.GetAttr("alias")
		if alias.IsKnown() && !alias.IsNull() && alias.Type() == cty.String {
			return alias.AsString(), true
		}
	case val.Type().IsTupleType() && val.LengthInt() == 2:
		// The [key, envID] form.
		if key := val.AsValueSlice()[0]; key.IsKnown() && key.Type() == cty.String {
			return key.AsString(), true
		}
	}
	return "", false
}

// AcceptCreatedBundleRef makes the newly created bundle the value of the input,
// which completes it.
func (w *BundleRefWidget) AcceptCreatedBundleRef(alias string) bool {
	w.PendingRefClass = ""
	w.value = cty.StringVal(alias)
	w.wctx.UpdateValue(w.value)
	return true
}

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
