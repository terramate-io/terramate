// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/typeschema"
)

// InputFocusArea represents which panel has focus in the split input view.
type InputFocusArea int

// InputFocusActive and the following constants define the input view focus areas.
const (
	InputFocusActive    InputFocusArea = iota // Top panel: current input / buttons
	InputFocusCompleted                       // Bottom panel: completed inputs
)

// InputsFormState represents the current state of the inputs form.
type InputsFormState int

// InputsFormActive and the following constants define the inputs form states.
const (
	InputsFormActive InputsFormState = iota
	InputsFormAccepted
	InputsFormDiscarded
	InputsFormCreateRef // Signals parent to push state and start nested bundle creation
	InputsFormSubForm   // Signals parent to open nested form
)

// InputsForm is a sub-model that handles sequential input forms in a scrollable canvas.
// Inputs are revealed one at a time. Previous answers are shown above.
// The user can navigate back to re-edit previous entries.
type InputsForm struct {
	Schemactx typeschema.EvalContext
	InputDefs []*config.InputDefinition

	activeIdx int // Index of the currently active input, or len(inputDefs) when on buttons

	// Original values for edit mode — baseline for detecting changes.
	originalValues map[string]cty.Value

	// Widgets keyed by input name. Each widget owns its value via WidgetContext.Value.
	widgets      map[string]InputWidget
	activeWidget InputWidget // Currently active widget (cached for hot path)
	preEditValue cty.Value   // Snapshot of the active input's value before editing began

	// Scrollable viewport
	viewport viewport.Model

	// browsing is true when the user is manually scrolling (Ctrl+U/D, PgUp/Dn).
	// While true, View() preserves the scroll position instead of snapping to bottom.
	browsing bool

	// Button selection when all inputs are done
	buttonIdx int // 0 = Add, 1 = Discard

	// Split panel focus state
	focus           InputFocusArea // Which panel currently has focus
	completedCursor int            // Cursor position in the completed inputs panel

	reconfiguring bool
	promoting     bool

	// Edit mode — true when editing an existing change (button label: Apply vs Add).
	editMode bool

	// Bundle reference support
	pendingRefClass string // Class to create when state is InputsFormCreateRef

	// Object input support
	pendingSubForm  *SubFormRequest // Set when the active widget signals WidgetNeedSubForm
	objectMode      bool            // When true, auto-confirm when all inputs are filled (no buttons/counter)
	isObjectForm    bool            // True when this form is editing object attributes (changes panel title)
	singleInputForm bool            // When true, hide the bottom panel (single synthetic input)
	parentTitle     string          // Title inherited from the parent form (used as fallback for subform panel title)

	// Discard confirmation (two-step: Discard -> "Are you sure?" Yes/No)
	confirmingDiscard bool           // True when the discard confirmation prompt is shown
	discardConfirmIdx int            // 0 = Yes, 1 = No
	preDiscardFocus   InputFocusArea // Focus to restore when discard is cancelled

	// Customizable button labels (empty = defaults: "Confirm" / "Cancel")
	confirmLabel string

	// Validation error shown at the top of the completed panel (e.g. from finalizeChange).
	validationErr error

	// Layout — dynamic panel dimensions; 0 means use defaults.
	PanelWidth  int
	PanelHeight int

	// Result
	state InputsFormState
}

// effectivePanelHeight returns the Height() budget for the input-form panel.
// When PanelHeight has been set (via WindowSizeMsg), it is used directly;
// otherwise a default derived from minContentHeight is returned.
func (f InputsForm) effectivePanelHeight() int {
	if f.PanelHeight > 0 {
		return f.PanelHeight
	}
	return minContentHeight + 6
}

// NewInputsForm creates a new inputs form for the given inputs and schemas
func NewInputsForm(inputDefs []*config.InputDefinition, schemactx typeschema.EvalContext, registry *config.Registry, env *config.Environment) InputsForm {
	vp := viewport.New(uiWidth, minContentHeight+6)
	vp.SetContent("")

	prompted := filterPrompted(inputDefs)

	widgets := make(map[string]InputWidget, len(prompted))

	shared := &SharedWidgetContext{
		Schemactx: schemactx,
		Registry:  registry,
		Env:       env,
	}

	for _, def := range prompted {
		wctx := &WidgetContext{
			SharedWidgetContext: shared,
			Width:               uiWidth,
			Def:                 def,
			Value:               cty.NilVal,
		}
		widgets[def.Name] = NewWidget(wctx, def.Type)
	}

	f := InputsForm{
		Schemactx: schemactx,
		InputDefs: prompted,
		activeIdx: 0,
		widgets:   widgets,
		viewport:  vp,
	}

	if len(prompted) > 0 {
		f.activeIdx = -1
		f.advanceToNextPending()
	}

	return f
}

// NewInputsFormWithValues creates an inputs form pre-populated with existing values.
func NewInputsFormWithValues(inputDefs []*config.InputDefinition, schemactx typeschema.EvalContext, registry *config.Registry, env, fromEnv *config.Environment, values, originalValues map[string]cty.Value) InputsForm {
	vp := viewport.New(uiWidth, minContentHeight+6)
	vp.SetContent("")

	prompted := filterPrompted(inputDefs)

	shared := &SharedWidgetContext{
		Schemactx: schemactx,
		Registry:  registry,
		Env:       env,
		FromEnv:   fromEnv,
	}

	widgets := make(map[string]InputWidget, len(prompted))
	for _, def := range prompted {
		val := cty.NilVal
		if v, ok := values[def.Name]; ok {
			val = v
		}
		wctx := &WidgetContext{
			SharedWidgetContext: shared,
			Width:               uiWidth,
			Def:                 def,
			Value:               val,
		}
		widgets[def.Name] = NewWidget(wctx, def.Type)
	}

	origSource := originalValues
	if origSource == nil {
		origSource = values
	}
	origCopy := make(map[string]cty.Value, len(prompted))
	for _, def := range prompted {
		if v, ok := origSource[def.Name]; ok {
			origCopy[def.Name] = v
		} else {
			origCopy[def.Name] = cty.NilVal
		}
	}

	f := InputsForm{
		Schemactx:       schemactx,
		InputDefs:       prompted,
		activeIdx:       len(prompted),
		originalValues:  origCopy,
		widgets:         widgets,
		viewport:        vp,
		editMode:        true,
		reconfiguring:   len(originalValues) > 0 && (fromEnv == nil || env == fromEnv),
		promoting:       len(originalValues) > 0 && (fromEnv != nil && env != fromEnv),
		focus:           InputFocusCompleted,
		completedCursor: 0,
	}

	f.buttonIdx = 0
	f.completedCursor = f.firstSelectableCursor()
	f.syncAllValuesToEvalctx()
	return f
}

// filterPrompted returns only the input definitions that have a prompt text set.
func filterPrompted(defs []*config.InputDefinition) []*config.InputDefinition {
	filtered := make([]*config.InputDefinition, 0, len(defs))
	for _, def := range defs {
		if def.Prompt.Text != "" {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// ReenterAt resets the form to the given input index, preserving values before
// that index and keeping existing values for later inputs so they pre-populate.
// The form resumes normal sequential flow from idx onward.
func (f *InputsForm) ReenterAt(idx int) {
	if idx < 0 || idx >= len(f.InputDefs) {
		return
	}
	f.activeIdx = idx
	f.focus = InputFocusActive
	f.confirmingDiscard = false
	f.prepareInput(idx)
}

// prepareInput configures the form controls for the given input index.
// The widget reads its existing value from ctx.Value during Prepare().
func (f *InputsForm) prepareInput(idx int) {
	if idx >= len(f.InputDefs) {
		return
	}
	def := f.InputDefs[idx]
	f.preEditValue = f.valueByName(def.Name)
	f.activeWidget = f.widgets[def.Name]
	f.activeWidget.Prepare()
}

// confirmCurrent advances to the next input after the widget has committed its value.
// The widget must have already called ctx.UpdateValue() successfully before
// signalling WidgetConfirmed.
// When the confirmed value differs from the pre-edit snapshot, all inputs that
// transitively depend on the changed input are cleared.
func (f *InputsForm) confirmCurrent() {
	if f.activeIdx >= len(f.InputDefs) || f.activeWidget == nil {
		return
	}
	editedIdx := f.activeIdx
	def := f.InputDefs[f.activeIdx]
	newVal := f.valueByName(def.Name)
	if !ctyValueEquals(f.preEditValue, newVal) {
		f.clearDependents(def.Name)
	}
	f.advanceToNextPending()

	// In reconfig/promote mode, return focus to the completed panel on the
	// just-edited input so the user can continue editing nearby inputs.
	if f.reconfiguring || f.promoting {
		visible := f.allVisibleIndices()
		for vi, idx := range visible {
			if idx == editedIdx {
				f.completedCursor = vi
				break
			}
		}
		f.focus = InputFocusCompleted
	}
}

// advanceToNextPending finds the first visible unfilled input whose dependencies
// are satisfied and makes it active. Inputs with pending dependencies are
// skipped (they remain visible but disabled).
// If all visible inputs are filled, the form moves to the done/buttons state.
func (f *InputsForm) advanceToNextPending() {
	for _, idx := range f.allVisibleIndices() {
		def := f.InputDefs[idx]
		if !f.isInputFilled(idx) && !f.hasPendingDependencies(def) {
			f.activeIdx = idx
			f.prepareInput(idx)
			return
		}
	}
	// All filled
	f.activeIdx = len(f.InputDefs)
	if f.objectMode {
		// In object mode, immediately signal completion without showing buttons.
		f.state = InputsFormAccepted
		return
	}
	f.buttonIdx = 0
	f.confirmingDiscard = false
}

// goBack moves to the previous visible input for re-editing, skipping
// conditional and dependency-disabled inputs.
func (f *InputsForm) goBack() bool {
	f.confirmingDiscard = false
	idx := f.activeIdx - 1
	for idx >= 0 {
		def := f.InputDefs[idx]
		if !f.evalCondition(def) || f.hasPendingDependencies(def) {
			idx--
			continue
		}
		f.activeIdx = idx
		f.prepareInput(f.activeIdx)
		return true
	}
	return false
}

// allInputsDone returns true when all inputs have been filled in.
func (f *InputsForm) allInputsDone() bool {
	return f.activeIdx >= len(f.InputDefs)
}

// ShowsTwoPanels returns true when the form is displaying the split two-panel layout.
func (f *InputsForm) ShowsTwoPanels() bool {
	return !f.allInputsDone()
}

// buttonsVisible returns true when the inline Confirm/Cancel buttons should be shown.
func (f *InputsForm) buttonsVisible() bool {
	return f.allInputsDone() && !f.objectMode
}

// HasPendingChanges returns true when any visible value differs from the original baseline.
// Only visible (condition-enabled) inputs are checked so that hidden inputs cleared
// as a side-effect of clearDependents do not keep buttons visible when the user
// reverts all visible values back to their originals.
// Always returns false when not in reconfiguring mode.
func (f *InputsForm) HasPendingChanges() bool {
	if !f.reconfiguring && !f.promoting {
		return false
	}
	for _, idx := range f.allVisibleIndices() {
		if f.isDefChanged(f.InputDefs[idx]) {
			return true
		}
	}
	return false
}

// isDefChanged returns true when the input's current value differs from the original.
func (f *InputsForm) isDefChanged(def *config.InputDefinition) bool {
	if f.originalValues == nil {
		return false
	}
	return !ctyValueEquals(f.valueByName(def.Name), f.originalValues[def.Name])
}

// HighlightedInputName returns the Name of the input currently under the cursor
// in the completed panel, or "" if nothing is highlighted.
func (f *InputsForm) HighlightedInputName() string {
	if f.focus != InputFocusCompleted {
		return ""
	}
	combined := f.allVisibleIndices()
	if f.completedCursor < 0 || f.completedCursor >= len(combined) {
		return ""
	}
	name := f.InputDefs[combined[f.completedCursor]].Name
	if isPseudoKey(name) {
		return ""
	}
	return name
}

// focusButtons switches focus to the button bar, selecting the first button.
func (f *InputsForm) focusButtons() {
	f.focus = InputFocusActive
	f.buttonIdx = 0
}

// firstSelectableCursor returns the first cursor index that is not immutable or disabled.
// Returns -1 if no selectable item exists (all inputs are immutable/disabled).
func (f *InputsForm) firstSelectableCursor() int {
	diffMode := f.reconfiguring || f.promoting
	visible := f.allVisibleIndices()
	for i, idx := range visible {
		def := f.InputDefs[idx]
		if (def.Immutable && diffMode) || f.hasPendingDependencies(def) {
			continue
		}
		return i
	}
	return -1
}

// hasSelectableInputs returns true when at least one input can be edited.
func (f *InputsForm) hasSelectableInputs() bool {
	return f.firstSelectableCursor() >= 0
}

// Remaining returns the number of visible inputs that have not been filled yet.
func (f *InputsForm) Remaining() int {
	count := 0
	for _, idx := range f.allVisibleIndices() {
		if !f.isInputFilled(idx) {
			count++
		}
	}
	return count
}

// hasHiddenConditionalInputs returns true if there are inputs with conditions
// that currently evaluate to false (hidden) and haven't been filled.
// These inputs may become visible once other values are provided.
func (f *InputsForm) hasHiddenConditionalInputs() bool {
	visibleSet := make(map[int]bool)
	for _, idx := range f.allVisibleIndices() {
		visibleSet[idx] = true
	}
	for i := range f.InputDefs {
		if !visibleSet[i] && !f.isInputFilled(i) {
			return true
		}
	}
	return false
}

// State returns the result state of the form.
func (f *InputsForm) State() InputsFormState {
	return f.state
}

// PendingRefClass returns the RefClass for InputsFormCreateRef state.
func (f *InputsForm) PendingRefClass() string {
	return f.pendingRefClass
}

// PendingSubForm returns the SubFormRequest from the active widget.
// Non-nil only when state is InputsFormEditObject.
func (f *InputsForm) PendingSubForm() *SubFormRequest {
	return f.pendingSubForm
}

// activeTitle returns the current top-panel title as a composed breadcrumb.
// When parentTitle is set and the active input has a PromptText, they are
// joined with " / " to maintain the navigation context.
func (f *InputsForm) activeTitle() string {
	base := f.parentTitle
	if f.activeIdx < len(f.InputDefs) && f.InputDefs[f.activeIdx].Prompt.Text != "" {
		prompt := f.InputDefs[f.activeIdx].Prompt.Text
		if base != "" {
			return base + " / " + prompt
		}
		return prompt
	}
	if base != "" {
		return base
	}
	return "Current Input"
}

// isPseudoKey returns true for reserved input names injected by the caller
// (e.g. "__output_name__", "__output_path__"). These are filtered out of
// Values() so they don't leak into bundle input maps.
func isPseudoKey(name string) bool {
	return strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")
}

// Values returns a copy of the collected input values, excluding synthetic
// keys injected by the caller.
func (f *InputsForm) Values() map[string]cty.Value {
	cp := make(map[string]cty.Value, len(f.widgets))
	for name, w := range f.widgets {
		cp[name] = w.WidgetContext().Value
	}
	return cp
}

// hasAnyValues returns true if the user has entered any non-pseudo values.
func (f *InputsForm) hasAnyValues() bool {
	for name, w := range f.widgets {
		if isPseudoKey(name) {
			continue
		}
		if w.WidgetContext().Value != cty.NilVal {
			return true
		}
	}
	return false
}

// IsConfirmingDiscard returns true if the discard confirmation prompt is active.
func (f *InputsForm) IsConfirmingDiscard() bool {
	return f.confirmingDiscard
}

// IsEditingInput returns true when the user is actively editing an individual input
// (as opposed to browsing the completed list or sitting on the action buttons).
func (f *InputsForm) IsEditingInput() bool {
	return f.focus == InputFocusActive && !f.allInputsDone()
}

// TriggerDiscard activates the discard confirmation prompt (same as pressing Cancel).
func (f *InputsForm) TriggerDiscard() {
	f.preDiscardFocus = f.focus
	f.confirmingDiscard = true
	f.discardConfirmIdx = 1
	f.focus = InputFocusActive
}

// SetValidationError sets an error to display at the top of the
// completed panel. The error is cleared automatically on the next key press.
func (f *InputsForm) SetValidationError(err error) {
	f.validationErr = err
}

// FocusActiveInput forwards a focus command to the active widget.
func (f *InputsForm) FocusActiveInput() tea.Cmd {
	if f.activeWidget != nil {
		return f.activeWidget.ForwardMsg(nil)
	}
	return nil
}

// IsMultilineActive returns true when the current input is a multiline textarea.
func (f *InputsForm) IsMultilineActive() bool {
	_, ok := f.activeWidget.(*MultilineWidget)
	return ok
}

// ForwardMsg forwards a non-key message to the active widget (cursor blink, etc).
func (f *InputsForm) ForwardMsg(msg tea.Msg) tea.Cmd {
	if f.activeWidget != nil {
		return f.activeWidget.ForwardMsg(msg)
	}
	return nil
}

// setBundleRefValue sets the current bundle-ref input to the given ID and advances.
func (f *InputsForm) setBundleRefValue(bundleID string) {
	if f.activeIdx >= len(f.InputDefs) {
		return
	}
	def := f.InputDefs[f.activeIdx]
	v := cty.StringVal(bundleID)
	f.setValueByName(def.Name, v)
	if bw, ok := f.activeWidget.(*BundleRefWidget); ok {
		bw.Reload()
	}
	f.state = InputsFormActive
	f.advanceToNextPending()
}

// syncAllValuesToEvalctx seeds the eval context with all current values at once.
func (f *InputsForm) syncAllValuesToEvalctx() {
	for name, w := range f.widgets {
		val := w.WidgetContext().Value
		if val != cty.NilVal {
			syncInputToEvalctx(f.Schemactx.Evalctx, name, val)
		}
	}
}

// valueByName returns the current value for the named input from its widget.
func (f *InputsForm) valueByName(name string) cty.Value {
	if w, ok := f.widgets[name]; ok {
		return w.WidgetContext().Value
	}
	return cty.NilVal
}

// setValueByName sets the value for the named input on its widget and syncs to eval context.
func (f *InputsForm) setValueByName(name string, val cty.Value) {
	if w, ok := f.widgets[name]; ok {
		w.WidgetContext().Value = val
		syncInputToEvalctx(f.Schemactx.Evalctx, name, val)
	}
}

// SeedValues sets initial values onto widgets without triggering side effects.
// Used when opening a sub-form with pre-existing values.
func (f *InputsForm) SeedValues(vals map[string]cty.Value) {
	for name, v := range vals {
		f.setValueByName(name, v)
	}
	if f.activeIdx >= 0 && f.activeIdx < len(f.InputDefs) {
		f.prepareInput(f.activeIdx)
	}
}

// evalCondition evaluates the prompt condition for a given input definition.
// Returns true (visible) when no condition is set or when it evaluates to true.
func (f *InputsForm) evalCondition(def *config.InputDefinition) bool {
	visible, _ := def.EvalPromptCondition(f.Schemactx)
	return visible
}

// hasPendingDependencies returns true when at least one input listed in
// def.Dependencies has not been filled yet. Such inputs should be shown
// but disabled (greyed out) until their dependencies are satisfied.
func (f *InputsForm) hasPendingDependencies(def *config.InputDefinition) bool {
	for depName := range def.Dependencies {
		if f.valueByName(depName) == cty.NilVal {
			return true
		}
	}
	return false
}

// clearDependents transitively unsets all inputs that depend on the given
// input name. For each cleared input, the value is set to NilVal and removed
// from the eval context so that downstream conditions/defaults see it as unset.
func (f *InputsForm) clearDependents(name string) {
	for _, def := range f.InputDefs {
		if _, ok := def.Dependencies[name]; !ok {
			continue
		}
		if f.valueByName(def.Name) == cty.NilVal {
			continue
		}
		f.setValueByName(def.Name, cty.NilVal)
		f.clearDependents(def.Name)
	}
}

// ctyValueEquals compares two cty.Value instances for equality,
// handling NilVal and Null gracefully.
func ctyValueEquals(a, b cty.Value) bool {
	if a == cty.NilVal && b == cty.NilVal {
		return true
	}
	if a == cty.NilVal || b == cty.NilVal {
		return false
	}
	if a.IsNull() && b.IsNull() {
		return true
	}
	if a.IsNull() || b.IsNull() {
		return false
	}
	eq := a.Equals(b)
	return eq.Type() == cty.Bool && eq.True()
}

// allVisibleIndices returns the indices of all inputs whose condition is nil
// or currently evaluates to true. This includes completed, active, and pending inputs.
func (f *InputsForm) allVisibleIndices() []int {
	var indices []int
	for i, def := range f.InputDefs {
		if f.evalCondition(def) {
			indices = append(indices, i)
		}
	}
	return indices
}

// isInputFilled returns true if the input at the given index has a value.
func (f *InputsForm) isInputFilled(idx int) bool {
	return f.valueByName(f.InputDefs[idx].Name) != cty.NilVal
}

// Update handles messages for the inputs form.
func (f InputsForm) Update(msg tea.Msg) (InputsForm, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		f.validationErr = nil

		// Tab switches focus between active and completed panels.
		// When all inputs are done (single panel) or single-input forms, Tab is a no-op.
		if msg.Type == tea.KeyTab {
			if f.allInputsDone() || f.singleInputForm {
				return f, nil
			}
			if f.focus == InputFocusActive {
				// Switch to inputs panel, positioning cursor on the active input
				visible := f.allVisibleIndices()
				if len(visible) > 0 {
					f.focus = InputFocusCompleted
					// Position cursor on the currently active input, or first selectable
					f.completedCursor = f.firstSelectableCursor()
					for vi, idx := range visible {
						if idx == f.activeIdx {
							f.completedCursor = vi
							break
						}
					}
					if f.completedCursor >= len(visible) {
						f.completedCursor = len(visible) - 1
					}
				}
			} else {
				f.focus = InputFocusActive
			}
			return f, nil
		}

		// When the completed panel is focused, handle navigation there.
		if f.focus == InputFocusCompleted {
			// Ctrl+U/D for scrolling
			switch msg.Type {
			case tea.KeyPgUp, tea.KeyCtrlU:
				f.viewport.HalfPageUp()
				f.browsing = true
				return f, nil
			case tea.KeyPgDown, tea.KeyCtrlD:
				f.viewport.HalfPageDown()
				f.browsing = true
				return f, nil
			}
			f.browsing = false
			return f.updateCompleted(msg)
		}

		// Active panel: Ctrl+U/D scroll the completed viewport
		switch msg.Type {
		case tea.KeyPgUp, tea.KeyCtrlU:
			f.viewport.HalfPageUp()
			f.browsing = true
			return f, nil
		case tea.KeyPgDown, tea.KeyCtrlD:
			f.viewport.HalfPageDown()
			f.browsing = true
			return f, nil
		}

		f.browsing = false
		if f.allInputsDone() {
			return f.updateButtons(msg)
		}
		return f.updateInput(msg)

	default:
		// Forward non-key messages (e.g. cursor blink) to the active widget.
		if f.focus == InputFocusActive && !f.allInputsDone() && f.activeWidget != nil {
			cmd := f.activeWidget.ForwardMsg(msg)
			return f, cmd
		}
	}

	return f, nil
}

// updateCompleted handles key messages when the inputs panel is focused.
func (f InputsForm) updateCompleted(msg tea.KeyMsg) (InputsForm, tea.Cmd) {
	combined := f.allVisibleIndices()
	if len(combined) == 0 {
		return f, nil
	}

	diffMode := f.reconfiguring || f.promoting

	isSkippable := func(idx int) bool {
		if idx < 0 || idx >= len(combined) {
			return false
		}
		def := f.InputDefs[combined[idx]]
		return (def.Immutable && diffMode) || f.hasPendingDependencies(def)
	}

	switch msg.Type {
	case tea.KeyUp:
		moved := false
		for next := f.completedCursor - 1; next >= 0; next-- {
			if !isSkippable(next) {
				f.completedCursor = next
				moved = true
				break
			}
		}
		if !moved {
			// Wrap to last selectable input
			for i := len(combined) - 1; i > f.completedCursor; i-- {
				if !isSkippable(i) {
					f.completedCursor = i
					break
				}
			}
		}
	case tea.KeyDown:
		moved := false
		for next := f.completedCursor + 1; next < len(combined); next++ {
			if !isSkippable(next) {
				f.completedCursor = next
				moved = true
				break
			}
		}
		if !moved && f.buttonsVisible() {
			f.focusButtons()
		}
	case tea.KeyDelete, tea.KeyBackspace:
		// Reset a changed input to its original value.
		if diffMode && f.completedCursor >= 0 && f.completedCursor < len(combined) {
			realIdx := combined[f.completedCursor]
			def := f.InputDefs[realIdx]
			if def.Immutable {
				return f, nil
			}
			if f.isDefChanged(def) {
				origVal := f.originalValues[def.Name]
				f.setValueByName(def.Name, origVal)
				newTotal := len(f.allVisibleIndices())
				if f.completedCursor >= newTotal && newTotal > 0 {
					f.completedCursor = newTotal - 1
				}
			}
		}
		return f, nil
	case tea.KeyEnter:
		if f.completedCursor < 0 || f.completedCursor >= len(combined) {
			return f, nil
		}
		realIdx := combined[f.completedCursor]
		def := f.InputDefs[realIdx]
		isImmutable := def.Immutable && diffMode
		if isImmutable || f.hasPendingDependencies(def) {
			return f, nil
		}
		f.ReenterAt(realIdx)
		f.focus = InputFocusActive
		return f, nil
	case tea.KeyEsc:
		f.focus = InputFocusActive
		return f, nil
	}
	return f, nil
}

// updateInput handles key messages when an input is active.
func (f InputsForm) updateInput(msg tea.KeyMsg) (InputsForm, tea.Cmd) {
	if f.activeWidget == nil {
		return f, nil
	}
	signal, cmd := f.activeWidget.Update(msg)

	switch signal {
	case WidgetConfirmed:
		f.confirmCurrent()
	case WidgetBack:
		f.goBack()
	case WidgetNeedSubForm:
		if bw, ok := f.activeWidget.(*BundleRefWidget); ok {
			f.pendingRefClass = bw.PendingRefClass
			f.state = InputsFormCreateRef
		} else if ow, ok := f.activeWidget.(*ObjectWidget); ok && ow.SubFormRequest != nil {
			f.pendingSubForm = ow.SubFormRequest
			f.state = InputsFormSubForm
		} else if lw, ok := f.activeWidget.(*SubFormListWidget); ok && lw.SubFormRequest != nil {
			f.pendingSubForm = lw.SubFormRequest
			f.state = InputsFormSubForm
		} else if mw, ok := f.activeWidget.(*SubFormMapWidget); ok && mw.SubFormRequest != nil {
			f.pendingSubForm = mw.SubFormRequest
			f.state = InputsFormSubForm
		}
	}

	return f, cmd
}

// updateButtons handles key messages when the action buttons are shown.
func (f InputsForm) updateButtons(msg tea.KeyMsg) (InputsForm, tea.Cmd) {
	if !f.buttonsVisible() {
		f.focus = InputFocusCompleted
		f.completedCursor = f.firstSelectableCursor()
		return f.updateCompleted(msg)
	}

	if f.confirmingDiscard {
		// Two-step: confirming discard (Yes / No)
		switch msg.Type {
		case tea.KeyLeft:
			if f.discardConfirmIdx > 0 {
				f.discardConfirmIdx--
			}
		case tea.KeyRight:
			if f.discardConfirmIdx < 1 {
				f.discardConfirmIdx++
			}
		case tea.KeyEnter:
			if f.discardConfirmIdx == 0 {
				f.state = InputsFormDiscarded
			} else {
				f.confirmingDiscard = false
				f.focus = f.preDiscardFocus
			}
		case tea.KeyEsc:
			f.confirmingDiscard = false
			f.focus = f.preDiscardFocus
		}
		return f, nil
	}

	// Normal state: Accept / Discard
	combined := f.allVisibleIndices()
	hasChanges := f.HasPendingChanges()

	// Button count: no changes in reconfig → 1 button (Back), otherwise → 2 (Save, Cancel)
	// Promote always has 2 buttons since it creates a new bundle.
	maxButton := 1 // 0=Save, 1=Cancel
	if f.reconfiguring && !hasChanges {
		maxButton = 0 // Only Back button at index 0
	}

	// lastSelectableCursor finds the last non-immutable, non-disabled input.
	lastSelectable := func() int {
		for i := len(combined) - 1; i >= 0; i-- {
			def := f.InputDefs[combined[i]]
			if (!def.Immutable || (!f.reconfiguring && !f.promoting)) && !f.hasPendingDependencies(def) {
				return i
			}
		}
		return f.firstSelectableCursor()
	}

	switch msg.Type {
	case tea.KeyUp:
		if f.buttonIdx > 0 {
			f.buttonIdx--
		} else {
			// Up from first button → back to last selectable input
			if len(combined) > 0 {
				f.focus = InputFocusCompleted
				f.completedCursor = lastSelectable()
			}
		}
	case tea.KeyDown:
		if f.buttonIdx < maxButton {
			f.buttonIdx++
		} else {
			// Down from last button → wrap to first selectable input
			if len(combined) > 0 {
				f.focus = InputFocusCompleted
				f.completedCursor = f.firstSelectableCursor()
			}
		}
	case tea.KeyLeft:
		if f.buttonIdx > 0 {
			f.buttonIdx--
		}
	case tea.KeyRight:
		if f.buttonIdx < maxButton {
			f.buttonIdx++
		}
	case tea.KeyShiftTab:
		f.goBack()
		return f, nil
	case tea.KeyEnter:
		if f.reconfiguring && !hasChanges {
			// Back button — discard without confirmation
			f.state = InputsFormDiscarded
		} else if f.buttonIdx == 0 {
			f.state = InputsFormAccepted
		} else {
			f.preDiscardFocus = f.focus
			f.confirmingDiscard = true
			f.discardConfirmIdx = 1 // Default to "No"
		}
	}
	return f, nil
}

// View renders the inputs form as two bordered panels:
//   - Top panel: the current active input (or action buttons when done)
//   - Bottom panel: already-completed inputs (scrollable, navigable when focused)
//
// Review mode (edit-change) renders a single review list instead.
func (f InputsForm) View() string {
	panelWidth := f.PanelWidth
	if panelWidth <= 0 {
		panelWidth = uiWidth
	}
	innerWidth := panelWidth - 6 // border (2) + padding (4)

	sectionTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	contentStyle := lipgloss.NewStyle().Width(innerWidth)

	focusedBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(panelWidth)

	unfocusedBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(panelWidth)

	botBorderOverhead := 4 // border (2) + padding (2)
	botTitleLines := 2     // section title + blank line

	// --- All inputs filled: single full-height panel with inline buttons ---
	if f.allInputsDone() {
		return f.renderSingleCompletedPanel(
			sectionTitleStyle, contentStyle, focusedBorder, innerWidth,
			botBorderOverhead, botTitleLines,
		)
	}

	// --- Top panel: active input ---
	counterStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)

	topTitle := f.activeTitle()
	if f.isObjectForm && f.activeIdx < len(f.InputDefs) && f.InputDefs[f.activeIdx].Prompt.Text != "" {
		topTitle = f.InputDefs[f.activeIdx].Prompt.Text
	}
	topTitleText := sectionTitleStyle.Render(topTitle)

	var counterText string
	if !f.objectMode {
		remaining := f.Remaining()
		if remaining > 0 {
			if f.hasHiddenConditionalInputs() {
				counterText = counterStyle.Render(fmt.Sprintf("~ %d inputs remaining", remaining))
			} else {
				counterText = counterStyle.Render(fmt.Sprintf("%d inputs remaining", remaining))
			}
		}
	}

	// Title left, counter right
	var topTitleLine string
	if counterText != "" {
		gap := innerWidth - lipgloss.Width(topTitleText) - lipgloss.Width(counterText)
		if gap < 1 {
			gap = 1
		}
		topTitleLine = topTitleText + strings.Repeat(" ", gap) + counterText
	} else {
		topTitleLine = topTitleText
	}

	activeContent := f.renderActiveSection()
	topInner := contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, topTitleLine, "", activeContent))

	var topPanel string
	if f.focus == InputFocusActive {
		topPanel = focusedBorder.Render(topInner)
	} else {
		topPanel = unfocusedBorder.Render(topInner)
	}

	if f.singleInputForm {
		return topPanel
	}

	// --- Bottom panel: all inputs ---
	botTitleText := "Inputs"
	if f.isObjectForm {
		if f.parentTitle != "" {
			botTitleText = f.parentTitle + " / Attributes"
		} else {
			botTitleText = "Attributes"
		}
	}
	botTitle := sectionTitleStyle.Render(botTitleText)
	completedContent := f.renderCompletedPanelContent()
	if completedContent == "" {
		dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)
		completedContent = dimStyle.Render("No inputs available")
	}

	// Compute available height for the bottom panel.
	topHeight := lipgloss.Height(topPanel)
	botMaxContent := f.effectivePanelHeight() - topHeight - botBorderOverhead - botTitleLines
	if botMaxContent < 1 {
		botMaxContent = 1
	}

	contentLines := lipgloss.Height(completedContent)
	var botInner string
	if contentLines <= botMaxContent {
		// Everything fits — render directly.
		botInner = contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, botTitle, "", completedContent))
	} else {
		// Overflow — use viewport for the completed list.
		// Reserve 2 chars for scrollbar gutter (space + scrollbar glyph).
		scrollbarGutter := 2
		f.viewport.Width = innerWidth - scrollbarGutter
		f.viewport.Height = botMaxContent
		f.viewport.SetContent(completedContent)

		if !f.browsing {
			// Determine which row to keep visible.
			focusRow := 0
			if f.focus == InputFocusCompleted {
				// Keep the cursor row visible when navigating the panel.
				focusRow = f.completedCursor
			} else {
				// Keep the active input row visible when the top panel has focus.
				visible := f.allVisibleIndices()
				for vi, idx := range visible {
					if idx == f.activeIdx {
						focusRow = vi
						break
					}
				}
			}
			if focusRow < f.viewport.YOffset {
				f.viewport.SetYOffset(focusRow)
			} else if focusRow >= f.viewport.YOffset+f.viewport.Height {
				f.viewport.SetYOffset(focusRow - f.viewport.Height + 1)
			}
		}

		vpView := f.viewport.View()
		scrollbar := f.renderScrollbar()
		if scrollbar != "" {
			vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
		}
		botInner = contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, botTitle, "", vpView))
	}

	// Fix the bottom panel height so the total layout stays consistent.
	botFixedHeight := f.effectivePanelHeight() - topHeight
	if botFixedHeight < 6 {
		botFixedHeight = 6
	}

	var bottomPanel string
	if f.focus == InputFocusCompleted {
		bottomPanel = focusedBorder.Height(botFixedHeight - botBorderOverhead).Render(botInner)
	} else {
		bottomPanel = unfocusedBorder.Height(botFixedHeight - botBorderOverhead).Render(botInner)
	}

	return lipgloss.JoinVertical(lipgloss.Left, topPanel, bottomPanel)
}

// renderSingleCompletedPanel renders the inputs panel at full height.
// When all inputs are done, the title line includes inline Accept/Discard buttons.
func (f InputsForm) renderSingleCompletedPanel(
	sectionTitleStyle, contentStyle, focusedBorder lipgloss.Style,
	innerWidth, borderOverhead, titleLines int,
) string {
	botTitleText := "Inputs"
	if f.isObjectForm {
		botTitleText = "Attributes"
	}
	titleRendered := sectionTitleStyle.Render(botTitleText)
	showButtons := f.allInputsDone() && !f.objectMode

	completedContent := f.renderCompletedPanelContent()
	if !f.hasSelectableInputs() && (f.reconfiguring || f.promoting) {
		noteStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)
		completedContent = completedContent + "\n\n" + noteStyle.Render("  All inputs are immutable — this bundle is not configurable.")
	}

	// Reserve space for buttons at the bottom (blank line + button row).
	buttonLines := 0
	if showButtons {
		buttonLines = 2
	}
	maxContent := f.effectivePanelHeight() - borderOverhead - titleLines - buttonLines
	if maxContent < 1 {
		maxContent = 1
	}

	contentLines := lipgloss.Height(completedContent)
	var contentBlock string
	if contentLines <= maxContent {
		contentBlock = contentStyle.Render(completedContent)
	} else {
		scrollbarGutter := 2
		f.viewport.Width = innerWidth - scrollbarGutter
		f.viewport.Height = maxContent
		f.viewport.SetContent(completedContent)

		if !f.browsing {
			focusLine := 0
			if f.completedCursor >= 0 {
				focusLine = f.completedCursor
			}
			if focusLine < f.viewport.YOffset {
				f.viewport.SetYOffset(focusLine)
			} else if focusLine >= f.viewport.YOffset+f.viewport.Height {
				f.viewport.SetYOffset(focusLine - f.viewport.Height + 1)
			}
		}

		vpView := f.viewport.View()
		scrollbar := f.renderScrollbar()
		if scrollbar != "" {
			vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
		}
		contentBlock = contentStyle.Render(vpView)
	}

	parts := []string{titleRendered, ""}
	if f.validationErr != nil {
		errStyle := validationStyle.PaddingLeft(2).Width(innerWidth)
		parts = append(parts, errStyle.Render(f.validationErr.Error()), "")
	}
	parts = append(parts, contentBlock)
	if showButtons {
		parts = append(parts, "", "  "+f.renderInlineButtons())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return focusedBorder.Height(f.effectivePanelHeight() - borderOverhead).Render(inner)
}

// renderScrollbar renders a vertical scrollbar track with a thumb.
func (f InputsForm) renderScrollbar() string {
	totalLines := f.viewport.TotalLineCount()
	visibleLines := f.viewport.VisibleLineCount()

	// No scrollbar needed if everything fits
	if totalLines <= visibleLines {
		return ""
	}

	trackHeight := f.viewport.Height
	if trackHeight < 1 {
		trackHeight = 1
	}

	// Calculate thumb size (minimum 1)
	thumbSize := (visibleLines * trackHeight) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	// Calculate thumb position
	scrollFraction := float64(f.viewport.YOffset) / float64(totalLines-visibleLines)
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

// summaryLine extracts the first paragraph of s (up to the first blank line),
// joining intermediate newlines with spaces — matching git's subject convention.
func summaryLine(s string) string {
	var parts []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			break
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " ")
}

// Styles used in rendering.
var (
	completedLabelStyle = lipgloss.NewStyle().Foreground(colorTextMuted)
	completedValueStyle = lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
	checkStyle          = lipgloss.NewStyle().Foreground(colorSecondary)

	activeDescStyle = lipgloss.NewStyle().Foreground(colorText)
	defaultStyle    = lipgloss.NewStyle().Foreground(colorTextMuted)
	promptStyle     = lipgloss.NewStyle().Foreground(colorPrimary)

	optionStyle       = lipgloss.NewStyle().PaddingLeft(2)
	activeOptionStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorPrimary).Bold(true)
	dimOptionStyle    = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorTextMuted)
	checkboxOn        = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	checkboxOff       = lipgloss.NewStyle().Foreground(colorTextMuted)

	boolActiveStyle   = lipgloss.NewStyle().Padding(0, 2).Background(colorPrimary).Foreground(lipgloss.Color("0")).Bold(true)
	boolInactiveStyle = lipgloss.NewStyle().Padding(0, 2).Background(colorBgSubtle).Foreground(colorText)

	validationStyle = lipgloss.NewStyle().Foreground(colorError)
)

// renderCompletedPanelContent renders all visible inputs in their natural order.
// Changed inputs are marked with a ~ icon and inline diff (new ← old).
// Immutable inputs show a blue value and "immutable" tag.
func (f InputsForm) renderCompletedPanelContent() string {
	combined := f.allVisibleIndices()
	if len(combined) == 0 {
		return ""
	}

	// Compute max label width for alignment.
	maxLabel := 0
	for _, realIdx := range combined {
		def := f.InputDefs[realIdx]
		label := def.Prompt.Text
		if len(label) > maxLabel {
			maxLabel = len(label)
		}
	}

	focused := f.focus == InputFocusCompleted
	selectedLabelStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	pendingLabelStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	disabledLabelStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Faint(true)
	activeLabelStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)
	changedIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	changedValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	immutableValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	wasStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	// Compute max value display width for tag alignment.
	maxValueWidth := 0
	for _, realIdx := range combined {
		if f.isInputFilled(realIdx) {
			w := len(f.formatCompletedValue(realIdx))
			if f.isDefChanged(f.InputDefs[realIdx]) {
				// Changed values also show " ← original"
				w += 3 + len(f.formatOriginalValue(realIdx))
			}
			if w > maxValueWidth {
				maxValueWidth = w
			}
		}
	}

	renderItem := func(cursorIdx int, realIdx int, isChanged bool) []string {
		def := f.InputDefs[realIdx]
		isCursorSelected := focused && cursorIdx == f.completedCursor
		isFilled := f.isInputFilled(realIdx)
		isActive := realIdx == f.activeIdx
		isDisabled := f.hasPendingDependencies(def)
		diffMode := f.reconfiguring || f.promoting
		isImmutable := def.Immutable && diffMode

		label := def.Prompt.Text
		labelPad := strings.Repeat(" ", maxLabel-len(label))

		var prefix, statusIcon, nameCol string

		if isImmutable {
			// Immutable: not selectable, always show tag
			prefix = "  "
			statusIcon = " "
			nameCol = completedLabelStyle.Render(label) + labelPad
		} else if isDisabled {
			prefix = "  "
			statusIcon = " "
			nameCol = disabledLabelStyle.Render(label) + labelPad
		} else if isCursorSelected {
			prefix = promptStyle.Render("›") + " "
			if isActive {
				statusIcon = selectedLabelStyle.Render("○")
			} else if isChanged {
				statusIcon = changedIconStyle.Render("~")
			} else if isFilled && !diffMode {
				statusIcon = checkStyle.Render("✓")
			} else {
				statusIcon = " "
			}
			nameCol = selectedLabelStyle.Render(label) + labelPad
		} else if isActive {
			prefix = "  "
			statusIcon = activeLabelStyle.Render("○")
			nameCol = activeLabelStyle.Render(label) + labelPad
		} else if isChanged {
			prefix = "  "
			statusIcon = changedIconStyle.Render("~")
			nameCol = completedLabelStyle.Render(label) + labelPad
		} else if isFilled && !diffMode {
			prefix = "  "
			statusIcon = checkStyle.Render("✓")
			nameCol = completedLabelStyle.Render(label) + labelPad
		} else if isFilled {
			prefix = "  "
			statusIcon = " "
			nameCol = completedLabelStyle.Render(label) + labelPad
		} else {
			prefix = "  "
			statusIcon = " "
			nameCol = pendingLabelStyle.Render(label) + labelPad
		}

		arrowStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

		// Build the value portion and compute its visible width for tag alignment.
		var valueStr string
		var valueWidth int
		if isImmutable && isFilled {
			v := f.formatCompletedValue(realIdx)
			valueStr = immutableValueStyle.Render(v)
			valueWidth = len(v)
		} else if isFilled && !isDisabled {
			if isChanged && diffMode {
				origDisplay := f.formatOriginalValue(realIdx)
				newDisplay := f.formatCompletedValue(realIdx)
				valueStr = changedValueStyle.Render(newDisplay) + " " + arrowStyle.Render("←") + " " + wasStyle.Render(origDisplay)
				valueWidth = len(newDisplay) + 3 + len(origDisplay)
			} else {
				v := f.formatCompletedValue(realIdx)
				valueStr = completedValueStyle.Render(v)
				valueWidth = len(v)
			}
		} else if !isFilled && !isDisabled && diffMode {
			origDisplay := f.formatOriginalValue(realIdx)
			valueStr = wasStyle.Render("?") + " " + arrowStyle.Render("←") + " " + wasStyle.Render(origDisplay)
			valueWidth = 1 + 3 + len(origDisplay)
		}

		// Build the base line.
		var line string
		if valueStr != "" {
			line = fmt.Sprintf("%s%s %s = %s", prefix, statusIcon, nameCol, valueStr)
		} else {
			line = fmt.Sprintf("%s%s %s", prefix, statusIcon, nameCol)
		}

		// Build the tag (immutable, enter to edit, etc.) and align to a common column.
		var tag string
		if isImmutable {
			tag = hintStyle.Render("immutable")
		} else if isCursorSelected && !isDisabled {
			tag = hintStyle.Render("enter to edit")
		} else if isCursorSelected && isDisabled {
			tag = hintStyle.Render("waiting for dependencies")
		}

		if tag != "" {
			pad := maxValueWidth - valueWidth
			if pad < 0 {
				pad = 0
			}
			line += strings.Repeat(" ", pad) + "  " + tag
		}

		return []string{line}
	}

	var lines []string
	for ci, realIdx := range combined {
		lines = append(lines, renderItem(ci, realIdx, f.isDefChanged(f.InputDefs[realIdx]))...)
	}

	return strings.Join(lines, "\n")
}

// renderActiveSection renders the current active input.
// Used as the content for the top panel in the split layout.
func (f InputsForm) renderActiveSection() string {
	if f.activeIdx < len(f.InputDefs) && f.activeWidget != nil {
		def := f.InputDefs[f.activeIdx]

		var lines []string
		if def.Description != "" {
			descStyle := activeDescStyle.PaddingLeft(2).Width(f.activeWidget.WidgetContext().Width)
			lines = append(lines, descStyle.Render(def.Description), "")
		}

		lines = append(lines, f.activeWidget.Render()...)

		return strings.Join(lines, "\n")
	}
	return ""
}

func (f InputsForm) formatCompletedValue(idx int) string {
	if idx < 0 || idx >= len(f.InputDefs) {
		return "<none>"
	}
	def := f.InputDefs[idx]
	w, ok := f.widgets[def.Name]
	if !ok {
		return "<none>"
	}
	if w.WidgetContext().Value == cty.NilVal {
		return "<none>"
	}
	return w.FormatDisplay()
}

func (f InputsForm) formatOriginalValue(idx int) string {
	def := f.InputDefs[idx]
	orig := f.originalValues[def.Name]
	if orig == cty.NilVal || orig.IsNull() {
		return "<none>"
	}
	return ctyToDisplayString(orig)
}

// renderInlineButtons returns compact inline buttons for the title line.
// In normal mode: [Confirm] [Cancel]
// In confirming-cancel mode: Entered values will be lost. Are you sure?  [Yes] [No]
// When the completed panel is focused, the highlighted button is dimmed.
func (f InputsForm) renderInlineButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSubtle).
		Foreground(colorText)

	selectedButtonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorPrimary).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	if f.confirmingDiscard {
		// Always show confirm prompt as active regardless of focus
		promptStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
		prompt := promptStyle.Render("Entered values will be lost. Are you sure?")

		var yesBtn, noBtn string
		if f.discardConfirmIdx == 0 {
			yesBtn = selectedButtonStyle.Render("Yes")
		} else {
			yesBtn = buttonStyle.Render("Yes")
		}
		if f.discardConfirmIdx == 1 {
			noBtn = selectedButtonStyle.Render("No")
		} else {
			noBtn = buttonStyle.Render("No")
		}
		return prompt + "  " + lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, " ", noBtn)
	}

	activeStyle := selectedButtonStyle
	if f.focus != InputFocusActive {
		activeStyle = buttonStyle
	}

	hasChanges := f.HasPendingChanges()

	confirmText := f.confirmLabel
	if confirmText == "" {
		confirmText = "Confirm"
	}
	cancelText := "Cancel"

	// No changes in reconfig: show only "Back" button (no Save, no Cancel)
	// Promote always shows Save since it creates a new bundle.
	if f.reconfiguring && !hasChanges {
		backText := "Back"
		if f.focus == InputFocusActive {
			return activeStyle.Render(backText)
		}
		return buttonStyle.Render(backText)
	}

	// Has changes: show both Save and Cancel
	var confirmBtn, cancelBtn string
	if f.buttonIdx == 0 {
		confirmBtn = activeStyle.Render(confirmText)
	} else {
		confirmBtn = buttonStyle.Render(confirmText)
	}
	if f.buttonIdx == 1 {
		cancelBtn = activeStyle.Render(cancelText)
	} else {
		cancelBtn = buttonStyle.Render(cancelText)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, confirmBtn, " ", cancelBtn)
}

const (
	pseudoKeyOutputName = "__output_name__"
	pseudoKeyOutputPath = "__output_path__"
)

// pseudoStringInput creates a synthetic string InputDefinition for use as
// an extra form field (e.g. instance name, output path) that is not part of
// the bundle's declared inputs.
func pseudoStringInput(name, title, description string) *config.InputDefinition {
	return &config.InputDefinition{
		Name:        name,
		Description: description,
		Type:        &typeschema.PrimitiveType{Name: "string"},
		Prompt:      config.PromptConfig{Text: title},
	}
}

// extractPseudoString reads a synthetic string value from a values map.
func extractPseudoString(values map[string]cty.Value, key string) string {
	if v, ok := values[key]; ok && v != cty.NilVal {
		return v.AsString()
	}
	return ""
}
