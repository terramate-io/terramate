// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateEnvSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState
	switch {
	case key.Matches(msg, keys.Up):
		if m.envCursor > 0 {
			m.envCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.envCursor < len(est.Registry.Environments)-1 {
			m.envCursor++
		}
	case key.Matches(msg, keys.Enter):
		m.selectedEnv = est.Registry.Environments[m.envCursor]
		m.viewState = ViewOverview
		return m, textarea.Blink
	}
	return m, nil
}

func (m Model) renderEnvSelectView() string {
	est := m.EngineState
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(uiWidth).
		Height(uiContentHeight + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	header := m.renderHeader("select environment")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		MarginBottom(1)

	descStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		MarginBottom(1)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(colorPrimary).
		Bold(true)

	itemDescStyle := lipgloss.NewStyle().
		PaddingLeft(4).
		Foreground(colorTextMuted)

	idStyle := lipgloss.NewStyle().
		Foreground(colorTextSubtle)

	title := titleStyle.Render("Select Environment")
	desc := descStyle.Render("Choose the target environment for this session")

	var items []string
	for i, env := range est.Registry.Environments {
		idTag := idStyle.Render("[" + env.ID + "]")
		if i == m.envCursor {
			items = append(items, selectedStyle.Render("› "+env.Name)+" "+idTag)
		} else {
			items = append(items, itemStyle.Render("  "+env.Name)+" "+idTag)
		}
		items = append(items, itemDescStyle.Render(env.Description))
		items = append(items, "")
	}

	panelContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{title, desc, ""}, items...)...)
	panel := borderStyle.Render(panelContent)
	help := helpStyle.Render(m.finalHelpText(""))

	content := lipgloss.JoinVertical(lipgloss.Left, header, panel, help)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}
