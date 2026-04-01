// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
)

func (m Model) updatePromoteSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.viewState = ViewOverview
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.promoteCursor > 0 {
			m.promoteCursor--
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.promoteCursor < len(m.promoteBundles)-1 {
			m.promoteCursor++
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if m.promoteCursor < len(m.promoteBundles) {
			if err := m.loadPromoteBundle(m.promoteBundles[m.promoteCursor]); err != nil {
				return m.updateError(err)
			}
			m.viewState = ViewPromoteInput
			return m, nil
		}
	}
	return m, nil
}

// loadPromoteBundle loads the bundle definition for the given bundle,
// evaluates its input definitions, and creates the inputs form pre-populated
// with the bundle's current values.
func (m *Model) loadPromoteBundle(b *config.Bundle) error {
	est := m.EngineState
	// We create a BundleDefinitionEntry
	bde := makeBundleDefinitionEntry(est.Root, b)

	schemactx, err := m.loadBundleEvalContext(bde)
	if err != nil {
		return err
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errors.E(err, "failed to evaluate input definitions")
	}

	values := inputsToValueMap(b.Inputs)

	m.promoteBundle = b
	m.selectedBundleDefEntry = bde
	m.inputsForm = NewInputsFormWithValues(inputDefs, schemactx, est.Registry, m.selectedEnv, b.Environment, values, values)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
	return nil
}

// buildPromoteBundles returns bundles from the promote_from environment that
// do not already have a counterpart (same alias) in the current environment.
func (m Model) buildPromoteBundles() []*config.Bundle {
	est := m.EngineState
	if m.selectedEnv == nil || m.selectedEnv.PromoteFrom == "" {
		return nil
	}
	promoteFrom := m.selectedEnv.PromoteFrom
	currentEnvID := m.selectedEnv.ID

	// Collect aliases that already exist in the current environment.
	existingAliases := make(map[string]bool)
	for _, b := range est.Registry.Bundles {
		if b.Environment != nil && b.Environment.ID == currentEnvID {
			existingAliases[b.Alias] = true
		}
	}

	var out []*config.Bundle
	for _, b := range est.Registry.Bundles {
		if b.Environment == nil || b.Environment.ID != promoteFrom {
			continue
		}
		if existingAliases[b.Alias] {
			continue
		}
		out = append(out, b)
	}
	return out
}

func envNameForID(envs []*config.Environment, envID string) string {
	for _, env := range envs {
		if env.ID == envID {
			return env.Name
		}
	}
	return ""
}

func (m Model) renderPromoteSelectView() string {
	est := m.EngineState

	promoteFromName := envNameForID(est.Registry.Environments, m.selectedEnv.PromoteFrom)

	panelWidth := m.effectiveWidth()
	innerWidth := panelWidth - 4
	scrollbarGutter := 4 // left gap(1) + scrollbar(1) + right gap(2)
	contentWidth := innerWidth - scrollbarGutter

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(panelWidth).
		Height(m.effectiveContentHeight() + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	contentStyle := lipgloss.NewStyle().Width(innerWidth)

	title := m.renderHeader("promote", panelWidth)

	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(colorText).MarginBottom(1).Render("Select a Bundle to Promote")
	desc := lipgloss.NewStyle().Foreground(colorTextMuted).MarginBottom(2).
		Render("These bundles from " + promoteFromName + " are not yet in " + m.selectedEnv.Name + ".")

	header := lipgloss.JoinVertical(lipgloss.Left, sectionTitle, desc, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := m.effectiveContentHeight() - headerHeight

	groups := groupBundles(m.promoteBundles)
	selectedGroupIdx, items := m.renderGroupedBundleItems(groups, m.promoteCursor, contentWidth)

	start, end := scrollWindowVar(selectedGroupIdx, items, availableHeight)

	var sb strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			sb.WriteString("\n\n")
		}
		sb.WriteString(items[i].content)
	}
	listContent := sb.String()

	if len(items) > end-start {
		trackHeight := lipgloss.Height(listContent)
		scrollbar := renderScrollbar(len(items), end-start, start, trackHeight)
		listContent = lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", scrollbar, "  ")
	}

	inner := contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, listContent))

	help := helpStyle.Render(m.finalHelpText("esc: back"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		borderStyle.Render(inner),
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m Model) renderPromoteInputView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	b := m.promoteBundle
	headerContext := fmt.Sprintf("promote / %s", b.Name)
	title := m.renderHeader(headerContext, panelWidth)

	formView := m.inputsForm.View()

	helpText := "esc: back"
	if m.inputsForm.ShowsTwoPanels() {
		helpText = "tab: switch section • esc: back"
	}
	help := helpStyle.Render(m.finalHelpText(helpText))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		formView,
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m Model) updatePromoteInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState

	if key.Matches(msg, keys.Escape) && !m.inputsForm.IsMultilineActive() {
		m.viewState = ViewPromoteSelect
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		change, err := NewPromoteChange(
			est, m.selectedEnv, m.promoteBundle, m.selectedBundleDefEntry,
			m.inputsForm.Schemactx, m.inputsForm.InputDefs, m.inputsForm.Values(),
		)
		if err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}
		m.SetPendingChanges(append(m.PendingChanges(), change))
		m.changesApplied = false

		m.viewState = ViewOverview
	case InputsFormDiscarded:
		m.viewState = ViewOverview
	}

	return m, cmd
}
