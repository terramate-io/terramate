// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
)

func (m Model) updateCreateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState
	if m.confirmingCreateExit {
		switch {
		case key.Matches(msg, keys.Left):
			if m.createExitConfirmIdx > 0 {
				m.createExitConfirmIdx--
			}
		case key.Matches(msg, keys.Right):
			if m.createExitConfirmIdx < 1 {
				m.createExitConfirmIdx++
			}
		case key.Matches(msg, keys.Enter):
			if m.createExitConfirmIdx == 0 {
				m.confirmingCreateExit = false
				m.objectEditStack = nil
				m.viewState = ViewOverview
				m.focus = FocusCommands
				return m, textarea.Blink
			}
			m.confirmingCreateExit = false
		case key.Matches(msg, keys.Escape):
			m.confirmingCreateExit = false
		}
		return m, nil
	}

	if key.Matches(msg, keys.Escape) && len(m.objectEditStack) > 0 && !m.inputsForm.IsMultilineActive() {
		if m.inputsForm.focus == InputFocusActive || m.inputsForm.allInputsDone() {
			subResult := SubFormResult{
				Values: m.inputsForm.Values(),
			}
			subDone := m.inputsForm.allInputsDone()
			frame := m.objectEditStack[len(m.objectEditStack)-1]
			m.objectEditStack = m.objectEditStack[:len(m.objectEditStack)-1]

			m.inputsForm = frame.inputsForm
			m.inputsForm.state = InputsFormActive
			if subDone {
				if m.inputsForm.activeWidget.AcceptSubFormResult(subResult) {
					m.inputsForm.confirmCurrent()
				}
			} else {
				switch m.inputsForm.activeWidget.(type) {
				case *SubFormListWidget, *SubFormMapWidget:
					// Cancelled sub-form for a list/map item — stay on the widget.
				default:
					m.inputsForm.prepareInput(m.inputsForm.activeIdx)
				}
			}
			return m, m.inputsForm.FocusActiveInput()
		}
	}

	if key.Matches(msg, keys.Escape) && m.inputsForm.focus == InputFocusActive && !m.inputsForm.IsMultilineActive() {
		if len(m.createStack) > 0 {
			m.restoreCreateFrame("")
			return m, nil
		}
		if m.inputsForm.hasAnyValues() {
			m.confirmingCreateExit = true
			m.createExitConfirmIdx = 1
			return m, nil
		}
		m.viewState = ViewCreateSelect
		m.bundleSelectPage = BundleSelectBundle
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		if len(m.objectEditStack) > 0 {
			subResult := SubFormResult{
				Values: m.inputsForm.Values(),
			}
			frame := m.objectEditStack[len(m.objectEditStack)-1]
			m.objectEditStack = m.objectEditStack[:len(m.objectEditStack)-1]
			m.inputsForm = frame.inputsForm
			m.inputsForm.state = InputsFormActive

			if m.inputsForm.activeWidget.AcceptSubFormResult(subResult) {
				m.inputsForm.confirmCurrent()
			}
			return m, m.inputsForm.FocusActiveInput()
		}

		change, err := NewCreateChange(
			est,
			m.selectedEnv,
			m.selectedBundleDefEntry,
			m.inputsForm.Schemactx,
			m.inputsForm.InputDefs,
			m.inputsForm.Values(),
		)
		if err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}

		m.SetPendingChanges(append(m.PendingChanges(), change))
		m.changesApplied = false
		m.lastUsedCollIdx = m.selectedCollIdx
		m.hasLastUsedColl = true

		if len(m.createStack) > 0 {
			m.restoreCreateFrame(change.Alias)
		} else {
			m.viewState = ViewOverview
		}
	case InputsFormDiscarded:
		if len(m.objectEditStack) > 0 {
			frame := m.objectEditStack[len(m.objectEditStack)-1]
			m.objectEditStack = m.objectEditStack[:len(m.objectEditStack)-1]
			m.inputsForm = frame.inputsForm
			m.inputsForm.state = InputsFormActive
			return m, m.inputsForm.FocusActiveInput()
		}

		if len(m.createStack) > 0 {
			m.restoreCreateFrame("")
		} else {
			m.viewState = ViewOverview
		}
	case InputsFormCreateRef:
		m.pushCreateFrame()
		if err := m.startNestedCreate(m.inputsForm.PendingRefClass()); err != nil {
			m.currentErr = err
			m.restoreCreateFrame("")
			return m, nil
		}
		return m, m.inputsForm.FocusActiveInput()
	case InputsFormSubForm:
		prevTitle := m.inputsForm.activeTitle()
		m.pushObjectEditFrame()
		req := m.inputsForm.PendingSubForm()

		schemactx := m.inputsForm.Schemactx.ChildContext()
		m.inputsForm = NewInputsForm(req.InputDefs, schemactx, est.Registry, m.selectedEnv)
		m.inputsForm.PanelWidth = m.effectiveWidth()
		m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
		if req.EditMode {
			m.inputsForm.SeedValues(req.Values)
		}
		m.inputsForm.objectMode = true
		m.inputsForm.isObjectForm = !req.SingleInput
		m.inputsForm.singleInputForm = req.SingleInput
		if req.Title != "" {
			m.inputsForm.parentTitle = prevTitle + " / " + req.Title
		} else {
			m.inputsForm.parentTitle = prevTitle
		}
		return m, m.inputsForm.FocusActiveInput()
	}

	return m, cmd
}

func setupExplicitBundleAlias(evalctx *eval.Context, bundleDef *hcl.DefineBundle) (string, error) {
	if bundleDef.Alias != nil {
		alias, err := config.EvalString(evalctx, bundleDef.Alias.Expr, "alias")
		if err != nil {
			return "", err
		}
		if alias == "" {
			return "", nil
		}

		var bundleVals map[string]cty.Value
		if ns, ok := evalctx.GetNamespace("bundle"); ok {
			bundleVals = ns.AsValueMap()
		}
		if bundleVals == nil {
			bundleVals = map[string]cty.Value{}
		}
		bundleVals["alias"] = cty.StringVal(alias)
		evalctx.SetNamespace("bundle", bundleVals)

		return alias, nil
	}
	return "", nil
}

func (m Model) renderCreateInputView() string {
	est := m.EngineState
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	collName := strings.ToLower(est.Collections[m.selectedCollIdx].Name)
	bundleName := ""
	if m.selectedBundleIdx < len(est.Collections[m.selectedCollIdx].Bundles) {
		bundleName = est.Collections[m.selectedCollIdx].Bundles[m.selectedBundleIdx].Name
	}

	var headerContext string
	if len(m.createStack) > 0 {
		parentName := m.createStack[len(m.createStack)-1].parentBundleName
		headerContext = fmt.Sprintf("add bundle / %s / %s (for %s)", collName, bundleName, parentName)
	} else {
		headerContext = fmt.Sprintf("add bundle / %s / %s", collName, bundleName)
	}

	title := m.renderHeader(headerContext, panelWidth)

	var help string
	if m.confirmingCreateExit {
		promptStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
		buttonStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorTextMuted)
		activeStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorPrimary).
			Foreground(lipgloss.Color("#000000")).
			Bold(true)

		prompt := promptStyle.Render("Entered values will be lost. Continue?")

		var yesBtn, noBtn string
		if m.createExitConfirmIdx == 0 {
			yesBtn = activeStyle.Render("Yes")
		} else {
			yesBtn = buttonStyle.Render("Yes")
		}
		if m.createExitConfirmIdx == 1 {
			noBtn = activeStyle.Render("No")
		} else {
			noBtn = buttonStyle.Render("No")
		}

		line := prompt + "  " + lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, " ", noBtn)
		help = helpStyle.Render(line)
	} else {
		helpText := "esc: back"
		if m.inputsForm.ShowsTwoPanels() {
			helpText = "tab: switch section • esc: back"
		}
		helpText = m.finalHelpText(helpText)

		if name := m.inputsForm.HighlightedInputName(); name != "" {
			hintRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Render("[" + name + "]")
			leftRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Render(helpText)
			gap := panelWidth + 2 - lipgloss.Width(leftRendered) - lipgloss.Width(hintRendered)
			if gap < 2 {
				gap = 2
			}
			help = leftRendered + strings.Repeat(" ", gap) + hintRendered
		} else {
			help = helpStyle.Render(helpText)
		}
	}

	section := m.renderInputsPage()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		section,
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m Model) renderInputsPage() string {
	return m.inputsForm.View()
}

// pushCreateFrame saves the current create state onto the stack.
func (m *Model) pushCreateFrame() {
	est := m.EngineState
	bundleName := est.Collections[m.selectedCollIdx].Bundles[m.selectedBundleIdx].Name
	frame := CreateFrame{
		bundleSelectPage:  m.bundleSelectPage,
		selectedCollIdx:   m.selectedCollIdx,
		selectedBundleIdx: m.selectedBundleIdx,
		inputsForm:        m.inputsForm,
		parentBundleName:  bundleName,
	}
	m.createStack = append(m.createStack, frame)
}

// restoreCreateFrame pops the last frame and restores the wizard state.
// If newBundleAlias is non-empty, the bundle-ref input is set to that value and advanced.
func (m *Model) restoreCreateFrame(newBundleAlias string) {
	if len(m.createStack) == 0 {
		return
	}
	frame := m.createStack[len(m.createStack)-1]
	m.createStack = m.createStack[:len(m.createStack)-1]
	m.viewState = ViewCreateInput
	m.bundleSelectPage = frame.bundleSelectPage
	m.selectedCollIdx = frame.selectedCollIdx
	m.selectedBundleIdx = frame.selectedBundleIdx
	m.inputsForm = frame.inputsForm
	m.inputsForm.state = InputsFormActive
	m.nestedRefClass = ""

	if newBundleAlias != "" {
		m.inputsForm.setBundleRefValue(newBundleAlias)
	}
}

// pushObjectEditFrame saves the current inputs form state for object input editing.
func (m *Model) pushObjectEditFrame() {
	def := m.inputsForm.InputDefs[m.inputsForm.activeIdx]
	m.objectEditStack = append(m.objectEditStack, ObjectEditFrame{
		inputsForm:    m.inputsForm,
		objectInputID: def.Name,
		objectName:    def.Name,
	})
}

// startNestedCreate scans collections for a bundle matching refClass, loads it,
// and enters the wizard. Falls back to the collection page if no match is found.
func (m *Model) startNestedCreate(refClass string) error {
	est := m.EngineState
	m.nestedRefClass = refClass
	for collIdx, coll := range est.Collections {
		for bundleIdx, bundle := range coll.Bundles {
			if bundle.Class == refClass {
				if err := m.loadBundleDef(collIdx, bundleIdx); err != nil {
					return err
				}
				m.viewState = ViewCreateInput
				return nil
			}
		}
	}
	m.viewState = ViewCreateSelect
	m.bundleSelectPage = BundleSelectCollection
	return errors.E("No bundle found for class %q", refClass)
}
