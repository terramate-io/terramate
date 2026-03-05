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
	"github.com/terramate-io/terramate/project"
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

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(uiWidth).
		Height(uiContentHeight + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	idStyle := lipgloss.NewStyle().
		Foreground(colorTextSubtle)

	contentStyle := lipgloss.NewStyle().Width(uiWidth - 4)

	title := m.renderHeader("promote")

	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(colorText).MarginBottom(1).Render("Select a Bundle to Promote")
	desc := lipgloss.NewStyle().Foreground(colorTextMuted).MarginBottom(2).
		Render("These bundles from " + promoteFromName + " are not yet in " + m.selectedEnv.Name + ".")

	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	headerNameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	fromStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	groups := groupBundles(m.promoteBundles)

	const colGap = 2

	type entryRow struct {
		left      string
		leftWidth int
		source    string
	}
	type groupRow struct {
		headerLeft      string
		headerLeftWidth int
		headerSource    string
		entries         []entryRow
	}

	// See renderReconfigSelectView
	groupRows := make([]groupRow, len(groups))
	globalMaxLeft := 0
	visualIdx := 0
	for gi, g := range groups {
		b0 := g.bundles[0]
		version := "v" + b0.DefinitionMetadata.Version

		headerLeft := "    " + headerNameStyle.Render(g.name) + " " + fromStyle.Render(version)
		headerLeftWidth := lipgloss.Width(headerLeft)
		if headerLeftWidth > globalMaxLeft {
			globalMaxLeft = headerLeftWidth
		}

		entries := make([]entryRow, len(g.bundles))
		for i, b := range g.bundles {
			filename := project.PrjAbsPath(est.Root.HostDir(), b.Info.HostPath()).String()
			isSelected := visualIdx == m.promoteCursor
			visualIdx++
			var left string
			displayName := displayNameFromAlias(b.Alias, b.Name)
			if isSelected {
				left = selectedStyle.Render("  › " + displayName)
			} else {
				left = "    " + displayName
			}
			if b.Environment != nil {
				idTag := idStyle.Render("[" + b.Environment.ID + "]")
				left += " " + idTag
			}

			w := lipgloss.Width(left)
			if w > globalMaxLeft {
				globalMaxLeft = w
			}
			entries[i] = entryRow{left: left, leftWidth: w, source: filename}
		}

		groupRows[gi] = groupRow{
			headerLeft:      headerLeft,
			headerLeftWidth: headerLeftWidth,
			headerSource:    b0.Source,
			entries:         entries,
		}
	}

	// Second pass: render lines with a unified source column.
	var lines []string
	for gi, gr := range groupRows {
		if gi > 0 {
			lines = append(lines, "")
		}

		headerPad := strings.Repeat(" ", globalMaxLeft-gr.headerLeftWidth+colGap)
		lines = append(lines, gr.headerLeft+headerPad+fromStyle.Render(gr.headerSource))

		for _, r := range gr.entries {
			entryPad := strings.Repeat(" ", globalMaxLeft-r.leftWidth+colGap)
			lines = append(lines, r.left+entryPad+fromStyle.Render(r.source))
		}
	}

	listContent := strings.Join(lines, "\n")
	inner := contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, sectionTitle, desc, listContent))

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
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	b := m.promoteBundle
	headerContext := fmt.Sprintf("promote / %s", b.Name)
	title := m.renderHeader(headerContext)

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
