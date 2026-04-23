// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
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
				m.viewState = m.createBackView()
				return m, textarea.Blink
			}
			m.confirmingCreateExit = false
		case key.Matches(msg, keys.Escape):
			m.confirmingCreateExit = false
		}
		return m, nil
	}

	if key.Matches(msg, keys.Escape) {
		if handled, cmd := m.trySubFormEscape(); handled {
			return m, cmd
		}
	}

	if key.Matches(msg, keys.Escape) && m.inputsForm.focus == InputFocusActive && !m.inputsForm.IsMultilineActive() {
		if len(m.createStack) > 0 {
			m.restoreCreateFrame("")
			return m, nil
		}
		if m.inputsForm.hasAnyValues() {
			m.confirmingCreateExit = true
			m.createExitConfirmIdx = 1 // default to "No" (don't discard)
			return m, nil
		}
		m.viewState = m.createBackView()
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	if handled, scmd := m.trySubFormStateTransition(m.selectedEnv); handled {
		return m, scmd
	}

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		change, err := NewCreateChange(
			est,
			m.selectedEnv,
			m.selectedBundleDefEntry,
			m.inputsForm.Schemactx,
			m.inputsForm.InputDefs,
			m.inputsForm.UserValues(),
		)
		if err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}

		// Save immediately — bundles must be in the registry for
		// reference resolution (nested bundles, alias evaluation).
		if err := change.Save(est.Registry.Environments); err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}
		if err := m.reloadAll(); err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}
		m.recordSessionChange(change)

		if len(m.createStack) > 0 {
			m.restoreCreateFrame(change.Alias)
		} else {
			m.selectedEnv = nil
			m.viewState = ViewOverview
		}
	case InputsFormDiscarded:
		if len(m.createStack) > 0 {
			m.restoreCreateFrame("")
		} else {
			m.viewState = m.createBackView()
		}
	case InputsFormCreateRef:
		m.pushCreateFrame()
		if err := m.startNestedCreate(m.inputsForm.PendingRefClass()); err != nil {
			m.restoreCreateFrame("")
			return m.updateError(err)
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
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	bundleName := ""
	if m.flatBundleCursor < len(m.flatBundles) {
		bundleName = m.flatBundles[m.flatBundleCursor].bundle.Name
	}

	var envTag string
	if m.selectedEnv != nil {
		envStyle := lipgloss.NewStyle().Foreground(colorPromote)
		envTag = envStyle.Render("[" + m.selectedEnv.Name + "]")
	} else {
		envStyle := lipgloss.NewStyle().Foreground(colorPromote)
		envTag = envStyle.Render("[Without Environment]")
	}

	var headerContext string
	if len(m.createStack) > 0 {
		// Build chain: Create ECS / Create VPC [staging]
		parts := make([]string, 0, len(m.createStack)+1)
		for _, frame := range m.createStack {
			parts = append(parts, "Create "+frame.parentBundleName)
		}
		parts = append(parts, "Create "+bundleName+" "+envTag)
		headerContext = strings.Join(parts, " / ")
	} else {
		headerContext = "Create " + bundleName + " " + envTag
	}

	title := m.renderHeader(headerContext)

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

		prompt := promptStyle.Render("Discard entered values?")

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
		if extra := m.inputsForm.ExtraHelpHints(); extra != "" {
			helpText += " • " + extra
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

// createBackView returns the correct view to navigate back to from the input form.
// For env-requiring bundles, goes back to env select. Otherwise, goes to bundle select.
func (m Model) createBackView() ViewState {
	if m.selectedBundleDefEntry != nil &&
		bundleRequiresEnv(m.EngineState.Evalctx, m.selectedBundleDefEntry.Define) &&
		len(m.EngineState.Registry.Environments) > 0 {
		return ViewCreateEnvSelect
	}
	return ViewCreateSelect
}

// pushCreateFrame saves the current create state onto the stack.
func (m *Model) pushCreateFrame() {
	bundleName := ""
	if m.flatBundleCursor < len(m.flatBundles) {
		bundleName = m.flatBundles[m.flatBundleCursor].bundle.Name
	}
	frame := CreateFrame{
		flatBundleCursor:       m.flatBundleCursor,
		selectedCollIdx:        m.selectedCollIdx,
		selectedBundleIdx:      m.selectedBundleIdx,
		selectedBundleDefEntry: m.selectedBundleDefEntry,
		selectedBundleSource:   m.selectedBundleSource,
		inputsForm:             m.inputsForm,
		parentBundleName:       bundleName,
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
	m.flatBundleCursor = frame.flatBundleCursor
	m.selectedCollIdx = frame.selectedCollIdx
	m.selectedBundleIdx = frame.selectedBundleIdx
	m.selectedBundleDefEntry = frame.selectedBundleDefEntry
	m.selectedBundleSource = frame.selectedBundleSource
	m.inputsForm = frame.inputsForm
	m.inputsForm.state = InputsFormActive
	m.nestedRefClass = ""

	if newBundleAlias != "" {
		m.inputsForm.setBundleRefValue(newBundleAlias)
	}
}

// startNestedCreate scans the flat bundle list for a bundle matching refClass,
// loads it, and enters the wizard. Falls back to the bundle list if no match is found.
func (m *Model) startNestedCreate(refClass string) error {
	// Nested bundles inherit the parent's environment — no env re-selection needed.
	m.nestedRefClass = refClass
	for i, entry := range m.flatBundles {
		if entry.bundle.Class == refClass {
			m.flatBundleCursor = i
			if err := m.loadBundleDef(entry.collIdx, entry.bundleIdx); err != nil {
				return err
			}
			m.viewState = ViewCreateInput
			return nil
		}
	}
	m.viewState = ViewCreateSelect
	return errors.E("No bundle found for class %q", refClass)
}
