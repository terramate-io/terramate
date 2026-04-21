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

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
)

func (m Model) updateOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear transient state on update.
	m.currentErr = nil
	m.lastSavedKey = ""

	sessionBundles := m.sessionBundles()

	// --- Commands / Summary focus ---
	switch {
	case key.Matches(msg, keys.Tab):
		switch m.focus {
		case FocusCommands:
			if len(sessionBundles) > 0 {
				m.focus = FocusSummary
			} else {
				m.focus = FocusCommands
				return m, textarea.Blink
			}
		case FocusSummary:
			m.focus = FocusCommands
			return m, textarea.Blink
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		switch m.focus {
		case FocusCommands:
			m.executeCommand()
			if m.cancelled {
				return m, tea.Quit
			}
		case FocusSummary:
			if m.summaryCursor < len(sessionBundles) {
				if err := m.loadReconfigBundle(sessionBundles[m.summaryCursor]); err != nil {
					return m.updateError(err)
				}
				m.reconfigFromOverview = true
				m.viewState = ViewReconfigInput
				return m, m.inputsForm.FocusActiveInput()
			}
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.focus == FocusSummary && m.summaryCursor > 0 {
			m.summaryCursor--
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.focus == FocusSummary && m.summaryCursor < len(sessionBundles)-1 {
			m.summaryCursor++
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.focus == FocusCommands {
			if m.commandIdx < len(m.commands)-1 {
				m.commandIdx++
			} else {
				m.commandIdx = 0
			}
		}
		return m, nil

	case key.Matches(msg, keys.Left):
		if m.focus == FocusCommands {
			if m.commandIdx > 0 {
				m.commandIdx--
			} else {
				m.commandIdx = len(m.commands) - 1
			}
		}
		return m, nil

	case key.Matches(msg, keys.Escape):
		return m, nil

	case m.focus == FocusCommands && m.selectCommandByKey(msg.String()):
		m.executeCommand()
		if m.cancelled {
			return m, tea.Quit
		}
		return m, nil

	}

	return m, nil
}

// executeCommand handles the currently selected command.
func (m *Model) executeCommand() {
	est := m.EngineState
	if m.commandIdx < 0 || m.commandIdx >= len(m.commands) {
		return
	}
	cmd := m.commands[m.commandIdx]

	switch cmd {
	case "Create":
		if len(est.Collections) == 0 {
			m.currentErr = errors.E("No collections available. Configure package sources to get started.")
			return
		}
		m.flatBundles = buildFlatBundles(est)
		if len(m.flatBundles) == 0 {
			m.currentErr = errors.E("No bundles available.")
			return
		}
		m.viewState = ViewCreateSelect
		m.flatBundleCursor = 0
		m.bundleSelectErr = ""
	case "Reconfigure":
		m.reconfigFilterPos = -1
		m.reconfigFilters = m.buildReconfigFilters()
		m.reconfigBundles = m.buildReconfigBundles()
		if len(m.reconfigBundles) == 0 {
			if len(est.Registry.Bundles) == 0 {
				m.currentErr = errors.E("No bundles found in the project. Add bundles first.")
			} else {
				m.currentErr = errors.E("No bundles available for reconfiguration.")
			}
			return
		}
		m.viewState = ViewReconfigSelect
		m.reconfigCursor = 0
	case "Promote":
		m.promoteFilterPos = -1
		m.promoteFilters = m.buildPromoteFilters()
		m.promoteBundles, m.promoteTargetEnvs = m.buildAllPromoteBundles()
		if len(m.promoteBundles) == 0 {
			if len(est.Registry.Environments) == 0 {
				m.currentErr = errors.E("This action requires environments, but none are configured.")
			} else {
				m.currentErr = errors.E("No bundles available for promotion.")
			}
			return
		}
		m.viewState = ViewPromoteSelect
		m.promoteCursor = 0
	case "Quit":
		m.cancelled = true
	}
}

func (m *Model) reloadAll() error {
	est := m.EngineState
	if err := est.CLI.Reload(est.Context); err != nil {
		return errors.E(err, "Failed to reload configuration.")
	}

	// We need to update this as reload will result in a new root config.
	est.Root = est.CLI.Engine().Config()

	newReg, err := engine.EvalProjectBundles(est.Root, est.ResolveAPI, est.Evalctx, true)
	if err != nil {
		return errors.E(err, "Failed to evaluate new bundles.")
	}

	// Update in-place so that existing pointers (e.g. SharedWidgetContext.Registry
	// in parent forms during nested creation) see the new data.
	*est.Registry = *newReg
	return nil
}

// sessionBundleKey returns a unique key for a bundle instance (host path + env ID).
func sessionBundleKey(hostPath string, env *config.Environment) string {
	if env != nil {
		return hostPath + "\x00" + env.ID
	}
	return hostPath
}

// recordSessionChange tracks a saved change for the session history panel and CLI exit log.
func (m *Model) recordSessionChange(c Change) {
	m.changeLog = append(m.changeLog, changeLogEntry(c))
	if m.sessionChanges == nil {
		m.sessionChanges = make(map[string][]ChangeKind)
	}
	key := sessionBundleKey(c.HostPath, c.Env)
	m.sessionChanges[key] = append(m.sessionChanges[key], c.Kind)
	m.lastSavedKey = key
}

// sessionBundles returns the registry bundles that were modified this session,
// sorted into grouped display order.
func (m Model) sessionBundles() []*config.Bundle {
	if len(m.sessionChanges) == 0 {
		return nil
	}
	var bundles []*config.Bundle
	for _, b := range m.EngineState.Registry.Bundles {
		if _, ok := m.sessionChanges[sessionBundleKey(b.Info.HostPath(), b.Environment)]; ok {
			bundles = append(bundles, b)
		}
	}
	groups := groupBundles(bundles)
	sorted := make([]*config.Bundle, 0, len(bundles))
	for _, g := range groups {
		sorted = append(sorted, g.bundles...)
	}
	return sorted
}

func changeLogEntry(c Change) string {
	var action string

	switch c.Kind {
	case ChangeCreate:
		action = "Created " + c.DisplayName
		if c.Env != nil {
			action += fmt.Sprintf(" [%s]", c.Env.ID)
		}
	case ChangeReconfig:
		action = "Reconfigured " + c.DisplayName
		if c.Env != nil {
			action += fmt.Sprintf(" [%s]", c.Env.ID)
		}
	case ChangePromote:
		action = fmt.Sprintf("Promoted %s from [%s] to [%s]", c.DisplayName, c.FromEnv.ID, c.Env.ID)
	default:
		panic("unsupported change kind " + c.Kind)
	}

	return fmt.Sprintf("%s at %s", action, c.HostPath)
}

func (m Model) renderOverviewView() string {
	panelWidth := m.effectiveWidth()
	innerWidth := panelWidth - 4

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText)

	focusedBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(panelWidth)

	unfocusedBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(panelWidth)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	contentStyle := lipgloss.NewStyle().
		Width(innerWidth)

	title := m.renderHeader("Select an Action")

	commandsGrid := m.renderCommandGrid(panelWidth)

	var mainContent string

	parts := []string{commandsGrid}
	if m.currentErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(colorError).PaddingLeft(4).Width(innerWidth)
		parts = append(parts, "", errStyle.Render(m.currentErr.Error()))
	}
	mainContent = contentStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)

	var mainSection string
	borderStyle := unfocusedBorderStyle
	if m.focus == FocusCommands {
		borderStyle = focusedBorderStyle
	}
	mainSection = borderStyle.Render(mainContent)

	sessionBundles := m.sessionBundles()
	showHistoryPanel := len(sessionBundles) > 0

	helpText := "c: create • r: reconfigure • p: promote • q: quit"
	if showHistoryPanel && m.focus == FocusSummary {
		helpText = "↑↓: navigate • enter: reconfigure • tab: switch section"
	} else if showHistoryPanel {
		helpText = "c: create • r: reconfigure • p: promote • q: quit • tab: switch section"
	}
	helpText = m.finalHelpText(helpText)

	hint := m.selectedCommandHint()
	var help string
	if hint != "" {
		hintRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true).Render(hint)
		leftRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Render(helpText)
		gap := panelWidth + 2 - lipgloss.Width(leftRendered) - lipgloss.Width(hintRendered)
		if gap < 2 {
			gap = 2
		}
		help = leftRendered + strings.Repeat(" ", gap) + hintRendered
	} else {
		help = helpStyle.Render(helpText)
	}

	var content string
	if showHistoryPanel {
		var summaryTitle string
		if m.focus == FocusSummary {
			summaryTitle = sectionTitleStyle.Render("Recently Changed — Select to Reconfigure")
		} else {
			summaryTitle = lipgloss.NewStyle().Foreground(colorTextMuted).Render("Recently Changed — Select to Reconfigure")
		}

		scrollbarGutter := 4
		contentWidth := innerWidth - scrollbarGutter
		groups := groupBundles(sessionBundles)
		selectedGroupIdx, items := m.renderSessionBundleItems(groups, m.summaryCursor, contentWidth)

		availableHeight := m.effectiveContentHeight()
		start, end := scrollWindowVar(selectedGroupIdx, items, availableHeight, 0)

		var sb strings.Builder
		for i := start; i < end; i++ {
			if i > start {
				sb.WriteByte('\n')
			}
			sb.WriteString(items[i].content)
		}
		listContent := sb.String()

		visibleCount := end - start
		if len(items) > visibleCount {
			trackHeight := lipgloss.Height(listContent)
			scrollbar := renderScrollbar(len(items), visibleCount, start, trackHeight)
			listContent = lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", scrollbar, "  ")
		}

		summaryFull := lipgloss.JoinVertical(lipgloss.Left,
			summaryTitle,
			"",
			contentStyle.Render(listContent),
		)

		var historySection string
		if m.focus == FocusSummary {
			historySection = focusedBorderStyle.Render(summaryFull)
		} else {
			historySection = unfocusedBorderStyle.Render(summaryFull)
		}

		content = lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			mainSection,
			historySection,
			help,
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			mainSection,
			help,
		)
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

// renderHeader renders the header with breadcrumbs.
func (m Model) renderHeader(context string) string {
	slashStyle := lipgloss.NewStyle().
		Foreground(colorText)

	contextStyle := lipgloss.NewStyle().
		Foreground(colorText)

	terramateStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary)

	left := terramateStyle.Render("Terramate UI")
	if context != "" {
		// Make the last breadcrumb segment bold
		parts := strings.Split(context, " / ")
		for i, part := range parts {
			left += slashStyle.Render(" / ")
			if i == len(parts)-1 {
				left += lipgloss.NewStyle().Bold(true).Foreground(colorText).Render(part)
			} else {
				left += contextStyle.Render(part)
			}
		}
	}

	return left
}

// selectCommandByKey matches a key press to a command's configured hotkey (case-insensitive).
// Returns true if a command was matched and commandIdx was updated.
func (m *Model) selectCommandByKey(key string) bool {
	key = strings.ToLower(key)
	for i, cmd := range m.commands {
		if cm, ok := commandMeta[cmd]; ok && cm.hotkey == key {
			m.commandIdx = i
			return true
		}
	}
	return false
}

// commandMeta maps each command name to its color, hotkey, and hint text.
var commandMeta = map[string]struct {
	color  lipgloss.AdaptiveColor
	hotkey string // keyboard shortcut to activate this command
	hint   string
}{
	"Create":      {color: colorCreate, hotkey: "c", hint: "Create infrastructure in your project"},
	"Reconfigure": {color: colorReconfig, hotkey: "r", hint: "Reconfigure existing infrastructure"},
	"Promote":     {color: colorPromote, hotkey: "p", hint: "Promote infrastructure to this environment"},
	"Quit":        {color: colorTextMuted, hotkey: "q", hint: "Quit Terramate"},
}

func (m Model) renderCommandGrid(_ int) string {
	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	selectedUnderline := selectedStyle.Underline(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)
	normalUnderline := normalStyle.Underline(true)

	// renderCmdWithHotkey underlines the hotkey letter within the command name.
	renderCmdWithHotkey := func(cmd, hotkey string, style, underlineStyle lipgloss.Style) string {
		if hotkey == "" {
			return style.Render(cmd)
		}
		// Find the hotkey letter position in the command name (case-insensitive).
		idx := strings.Index(strings.ToLower(cmd), strings.ToLower(hotkey))
		if idx < 0 {
			return style.Render(cmd)
		}
		return style.Render(cmd[:idx]) + underlineStyle.Render(cmd[idx:idx+len(hotkey)]) + style.Render(cmd[idx+len(hotkey):])
	}

	cmds := m.commands
	if len(cmds) == 0 {
		return ""
	}

	const buttonGap = 6

	var rowStr string
	for i, cmd := range cmds {
		isSelected := m.focus == FocusCommands && i == m.commandIdx
		cm := commandMeta[cmd]

		var rendered string
		if isSelected {
			indicator := lipgloss.NewStyle().Foreground(cm.color).Render("›")
			rendered = fmt.Sprintf("  %s %s", indicator, renderCmdWithHotkey(cmd, cm.hotkey, selectedStyle, selectedUnderline))
		} else {
			rendered = fmt.Sprintf("    %s", renderCmdWithHotkey(cmd, cm.hotkey, normalStyle, normalUnderline))
		}

		if i > 0 {
			rowStr += strings.Repeat(" ", buttonGap)
		}
		rowStr += rendered
	}

	return rowStr
}

// selectedCommandHint returns the hint text for the currently selected command button, or "".
func (m Model) selectedCommandHint() string {
	if m.focus != FocusCommands || m.commandIdx < 0 || m.commandIdx >= len(m.commands) {
		return ""
	}
	if cm, ok := commandMeta[m.commands[m.commandIdx]]; ok {
		return cm.hint
	}
	return ""
}

// renderSessionBundleItems renders grouped session bundles with change kind tags.
func (m Model) renderSessionBundleItems(groups []bundleGroup, cursor, contentWidth int) (int, []renderedItem) {
	focused := m.focus == FocusSummary

	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	var selectedStyle, headerNameStyle, versionStyle, envStyle lipgloss.Style
	if focused {
		selectedStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
		headerNameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText)
		versionStyle = lipgloss.NewStyle().Foreground(colorTextSubtle)
		envStyle = lipgloss.NewStyle().Foreground(colorTextMuted)
	} else {
		selectedStyle = dimStyle
		headerNameStyle = dimStyle
		versionStyle = dimStyle
		envStyle = dimStyle
	}

	lineStyle := lipgloss.NewStyle().Width(contentWidth)

	kindLabels := map[ChangeKind]struct {
		label string
		color lipgloss.AdaptiveColor
	}{
		ChangeCreate:   {label: "created", color: colorCreate},
		ChangeReconfig: {label: "reconfigured", color: colorReconfig},
		ChangePromote:  {label: "promoted", color: colorPromote},
	}

	var items []renderedItem
	selectedItemIdx := 0
	visualIdx := 0

	for gi, g := range groups {
		b0 := g.bundles[0]

		if gi > 0 {
			items = append(items, renderedItem{content: "", height: 1})
		}

		headerLine := headerNameStyle.Render(g.name) + " " + versionStyle.Render("v"+b0.DefinitionMetadata.Version)
		items = append(items, renderedItem{content: lineStyle.Render(headerLine), height: 1})

		for _, b := range g.bundles {
			isSelected := focused && visualIdx == cursor
			if isSelected {
				selectedItemIdx = len(items)
			}
			visualIdx++

			displayName := displayNameFromAlias(b.Alias, b.Name)
			key := sessionBundleKey(b.Info.HostPath(), b.Environment)
			isLastSaved := m.lastSavedKey != "" && key == m.lastSavedKey

			var line string
			if isSelected {
				line = selectedStyle.Render("  › " + displayName)
			} else if isLastSaved {
				savedStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
				line = "    " + savedStyle.Render(displayName)
			} else if focused {
				line = "    " + displayName
			} else {
				line = "    " + dimStyle.Render(displayName)
			}
			if b.Environment != nil {
				line += " " + envStyle.Render("["+b.Environment.Name+"]")
			}

			// Append change kind tags
			if kinds, ok := m.sessionChanges[key]; ok {
				seen := make(map[ChangeKind]bool)
				for _, k := range kinds {
					if seen[k] {
						continue
					}
					seen[k] = true
					if kl, ok := kindLabels[k]; ok {
						var tagStyle lipgloss.Style
						if focused {
							tagStyle = lipgloss.NewStyle().Foreground(kl.color)
						} else {
							tagStyle = dimStyle
						}
						line += " " + tagStyle.Render(kl.label)
					}
				}
			}

			if isLastSaved {
				savedTagStyle := lipgloss.NewStyle().Foreground(colorCreate).Italic(true)
				line += " " + savedTagStyle.Render("✓ saved")
			}

			items = append(items, renderedItem{content: lineStyle.Render(line), height: 1})
		}
	}

	return selectedItemIdx, items
}
