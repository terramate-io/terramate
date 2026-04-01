// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmingEditDiscard {
		switch {
		case key.Matches(msg, keys.Left):
			if m.editDiscardConfirmIdx > 0 {
				m.editDiscardConfirmIdx--
			}
		case key.Matches(msg, keys.Right):
			if m.editDiscardConfirmIdx < 1 {
				m.editDiscardConfirmIdx++
			}
		case key.Matches(msg, keys.Enter):
			if m.editDiscardConfirmIdx == 0 {
				m.confirmingEditDiscard = false
				m.viewState = ViewOverview
				return m, nil
			}
			m.confirmingEditDiscard = false
		case key.Matches(msg, keys.Escape):
			m.confirmingEditDiscard = false
		}
		return m, nil
	}

	if key.Matches(msg, keys.Escape) && !m.inputsForm.IsMultilineActive() {
		m.confirmingEditDiscard = true
		m.editDiscardConfirmIdx = 1
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		oldChange := m.PendingChanges()[m.editingChangeIdx]

		if oldChange.Kind == ChangeReconfig && !m.inputsForm.HasPendingChanges() {
			// There are no more pending changes in this reconfigure form. We can just drop the change.
			pcs := m.PendingChanges()
			pcs = append(pcs[:m.editingChangeIdx], pcs[m.editingChangeIdx+1:]...)
			m.SetPendingChanges(pcs)

		} else {
			m.PendingChanges()[m.editingChangeIdx].MarkedForReplacement = true

			// We throw the old change away.
			change, err := NewChangeFromExisting(
				m.EngineState,
				oldChange,
				m.inputsForm.Schemactx,
				m.inputsForm.InputDefs,
				m.inputsForm.Values(),
			)
			if err != nil {
				m.PendingChanges()[m.editingChangeIdx].MarkedForReplacement = false
				m.inputsForm.SetValidationError(err)
				m.inputsForm.state = InputsFormActive
				break
			}
			m.PendingChanges()[m.editingChangeIdx] = change
		}

		m.confirmingEditDiscard = false
		m.viewState = ViewOverview
		if len(m.PendingChanges()) == 0 && m.focus == FocusSummary {
			m.focus = FocusCommands
		}
	case InputsFormDiscarded:
		m.confirmingEditDiscard = false
		m.viewState = ViewOverview
	}

	return m, cmd
}

// openEditChange transitions to the edit-change view for the given change index.
func (m *Model) openEditChange(idx int) error {
	if idx < 0 || idx >= len(m.PendingChanges()) {
		return nil
	}
	change := m.PendingChanges()[idx]
	m.editingChangeIdx = idx

	schemactx, err := m.loadBundleEvalContext(change.BundleDefEntry)
	if err != nil {
		return err
	}
	m.inputsForm = NewInputsFormWithValues(
		change.InputDefs, schemactx, m.EngineState.Registry, change.Env, change.FromEnv,
		change.Values, change.OriginalValues,
	)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()

	m.viewState = ViewEdit
	return nil
}

func (m Model) renderEditChangeView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	change := m.PendingChanges()[m.editingChangeIdx]
	headerContext := fmt.Sprintf("edit bundle / %s", change.Name)
	title := m.renderHeader(headerContext, panelWidth)

	formView := m.inputsForm.View()

	var help string
	if m.confirmingEditDiscard {
		promptStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
		buttonStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorTextMuted)
		activeStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorPrimary).
			Foreground(lipgloss.Color("#000000")).
			Bold(true)

		prompt := promptStyle.Render("Discard changes to this bundle?")

		var yesBtn, noBtn string
		if m.editDiscardConfirmIdx == 0 {
			yesBtn = activeStyle.Render("Yes")
		} else {
			yesBtn = buttonStyle.Render("Yes")
		}
		if m.editDiscardConfirmIdx == 1 {
			noBtn = activeStyle.Render("No")
		} else {
			noBtn = buttonStyle.Render("No")
		}

		line := prompt + "  " + lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, " ", noBtn)
		help = helpStyle.Render(line)
	} else {
		helpText := "esc: cancel"
		if m.inputsForm.ShowsTwoPanels() {
			helpText = "tab: switch section • esc: cancel"
		}
		help = helpStyle.Render(m.finalHelpText(helpText))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		formView,
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}
