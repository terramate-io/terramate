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

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
)

func (m Model) updateOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState

	// Clear transient errors on update.
	m.currentErr = nil
	m.saveErr = nil

	// Ctrl+E opens environment selector from any focus area
	if msg.String() == "ctrl+e" && len(est.Registry.Environments) > 0 {
		m.viewState = ViewEnvSelect
		return m, nil
	}

	// --- Exit confirmation ---
	if m.confirmingExit {
		switch {
		case key.Matches(msg, keys.Left):
			if m.exitConfirmIdx > 0 {
				m.exitConfirmIdx--
			}
		case key.Matches(msg, keys.Right):
			if m.exitConfirmIdx < 1 {
				m.exitConfirmIdx++
			}
		case key.Matches(msg, keys.Enter):
			if m.exitConfirmIdx == 0 {
				m.cancelled = true
				return m, tea.Quit
			}
			m.confirmingExit = false
		case key.Matches(msg, keys.Escape):
			m.confirmingExit = false
		}
		return m, nil
	}

	// --- Discard confirmation ---
	if m.confirmingDiscard {
		switch {
		case key.Matches(msg, keys.Left):
			if m.discardConfirmIdx > 0 {
				m.discardConfirmIdx--
			}
		case key.Matches(msg, keys.Right):
			if m.discardConfirmIdx < 1 {
				m.discardConfirmIdx++
			}
		case key.Matches(msg, keys.Enter):
			if m.discardConfirmIdx == 0 {
				m.SetPendingChanges(nil)
				m.summaryCursor = 0
				m.summaryOnButtons = false
				m.confirmingDiscard = false
				m.focus = FocusCommands
			} else {
				m.confirmingDiscard = false
			}
		case key.Matches(msg, keys.Escape):
			m.confirmingDiscard = false
		}
		return m, nil
	}

	// --- Commands / Summary focus ---
	switch {
	case key.Matches(msg, keys.Tab):
		switch m.focus {
		case FocusCommands:
			if len(m.PendingChanges()) > 0 || m.changesApplied {
				m.focus = FocusSummary
				m.summaryOnButtons = true
				m.summaryButtonIdx = 0
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
		if m.focus == FocusCommands {
			m.executeCommand()
			if m.cancelled {
				return m, tea.Quit
			}
		} else if m.focus == FocusSummary {
			if m.changesApplied {
				m.changesApplied = false
				m.savedChanges = nil
				m.focus = FocusCommands
			} else if m.summaryOnButtons && len(m.PendingChanges()) > 0 {
				if m.summaryButtonIdx == 0 {
					if err := m.saveChanges(); err != nil {
						m.saveErr = err
						return m, nil
					}
				} else {
					m.confirmingDiscard = true
					m.discardConfirmIdx = 1
				}
			} else if len(m.PendingChanges()) > 0 {
				if err := m.openEditChange(m.summaryCursor); err != nil {
					m.currentErr = err
					return m, nil
				}
				return m, m.inputsForm.FocusActiveInput()
			}
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.focus == FocusSummary {
			if m.summaryOnButtons && len(m.PendingChanges()) > 0 {
				m.summaryOnButtons = false
				m.summaryCursor = len(m.PendingChanges()) - 1
			} else if m.summaryCursor > 0 {
				m.summaryCursor--
			} else if len(m.PendingChanges()) > 0 {
				m.summaryOnButtons = true
			}
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.focus == FocusSummary {
			if m.summaryOnButtons && len(m.PendingChanges()) > 0 {
				m.summaryOnButtons = false
				m.summaryCursor = 0
			} else if !m.summaryOnButtons && m.summaryCursor < len(m.PendingChanges())-1 {
				m.summaryCursor++
			} else if !m.summaryOnButtons && m.summaryCursor >= len(m.PendingChanges())-1 {
				m.summaryOnButtons = true
			}
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.focus == FocusSummary && m.summaryOnButtons {
			if m.summaryButtonIdx < 1 {
				m.summaryButtonIdx++
			}
		} else if m.focus == FocusSummary && len(m.PendingChanges()) > 0 {
			m.summaryOnButtons = true
			m.summaryButtonIdx = 0
		} else if m.focus == FocusCommands {
			if m.commandIdx < len(m.commands)-1 {
				m.commandIdx++
			} else {
				m.commandIdx = 0
			}
		}
		return m, nil

	case key.Matches(msg, keys.Left):
		if m.focus == FocusSummary && m.summaryOnButtons {
			if m.summaryButtonIdx > 0 {
				m.summaryButtonIdx--
			} else {
				if len(m.PendingChanges()) > 0 {
					m.summaryOnButtons = false
				}
			}
		} else if m.focus == FocusCommands {
			if m.commandIdx > 0 {
				m.commandIdx--
			} else {
				m.commandIdx = len(m.commands) - 1
			}
		}
		return m, nil

	case key.Matches(msg, keys.Escape):
		if m.focus == FocusSummary && !m.summaryOnButtons {
			m.summaryOnButtons = true
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
		m.viewState = ViewCreateSelect
		m.bundleSelectPage = BundleSelectCollection
		if m.hasLastUsedColl {
			m.selectedCollIdx = m.lastUsedCollIdx
		} else {
			m.selectedCollIdx = 0
		}
		m.selectedBundleIdx = 0
	case "Reconfigure":
		m.reconfigBundles = m.buildReconfigBundles()
		if len(m.reconfigBundles) == 0 {
			if len(est.Registry.Bundles) == 0 {
				m.currentErr = errors.E("No bundles found in the project. Add bundles first.")
			} else {
				m.currentErr = errors.E("All bundles already have pending changes.")
			}
			return
		}
		m.viewState = ViewReconfigSelect
		m.reconfigCursor = 0
	case "Promote":
		if m.selectedEnv == nil {
			m.currentErr = errors.E("This action requires environments, but none are configured.")
			return
		}
		if m.selectedEnv.PromoteFrom == "" {
			m.currentErr = errors.E("The current environment does not have promote_from configured.")
			return
		}
		m.promoteBundles = m.buildPromoteBundles()
		if len(m.promoteBundles) == 0 {
			m.currentErr = errors.E("No bundles available to promote from %s.", m.selectedEnv.PromoteFrom)
			return
		}
		m.viewState = ViewPromoteSelect
		m.promoteCursor = 0
	case "Quit":
		if len(m.PendingChanges()) > 0 {
			m.confirmingExit = true
			m.exitConfirmIdx = 1
		} else {
			m.cancelled = true
		}
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

	est.Registry.Registry = newReg
	return nil
}

// saveChanges persists all pending changes to disk and reloads them.
// Successfully saved changes are moved to the change log even on partial failure.
func (m *Model) saveChanges() error {
	est := m.EngineState

	errs := errors.L()
	var saved []SavedChange
	var failed []Change

	for i, c := range m.PendingChanges() {
		err := m.PendingChanges()[i].Save(est.Registry.Environments)
		if err != nil {
			errs.Append(err)
			failed = append(failed, c)
		} else {
			var envID, fromEnvID string
			if c.Env != nil {
				envID = c.Env.ID
			}
			if c.FromEnv != nil {
				fromEnvID = c.FromEnv.ID
			}
			saved = append(saved, SavedChange{
				Kind:        c.Kind,
				Name:        c.DisplayName,
				HostPath:    c.HostPath,
				ProjectPath: c.ProjectPath,
				EnvID:       envID,
				FromEnvID:   fromEnvID,
			})
			m.changeLog = append(m.changeLog, changeLogEntry(c))
		}
	}

	m.savedChanges = saved
	errs.Append(m.reloadAll())

	m.changesApplied = len(saved) > 0
	m.SetPendingChanges(failed)
	m.summaryCursor = 0
	m.summaryOnButtons = false
	return errs.AsError()
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
	est := m.EngineState
	innerWidth := uiWidth - 4

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText)

	focusedBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(uiWidth)

	unfocusedBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(uiWidth)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	contentStyle := lipgloss.NewStyle().
		Width(innerWidth)

	title := m.renderHeader("overview")

	commandsGrid := m.renderCommandGrid()

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

	showSummaryPanel := len(m.PendingChanges()) > 0 || m.changesApplied

	hasEnvs := len(est.Registry.Environments) > 0
	envHint := ""
	if hasEnvs {
		envHint = "ctrl+e: environment • "
	}
	helpText := envHint
	if m.changesApplied && m.focus == FocusSummary {
		helpText = "enter: clear • tab: switch section • " + envHint
	} else if showSummaryPanel {
		helpText = "tab: switch section • " + envHint
	}
	helpText = m.finalHelpText(strings.TrimSuffix(helpText, " • "))

	hint := m.selectedCommandHint()
	var help string
	if hint != "" {
		hintRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true).Render(hint)
		leftRendered := lipgloss.NewStyle().Foreground(colorTextMuted).Render(helpText)
		gap := uiWidth + 2 - lipgloss.Width(leftRendered) - lipgloss.Width(hintRendered)
		if gap < 2 {
			gap = 2
		}
		help = leftRendered + strings.Repeat(" ", gap) + hintRendered
	} else {
		help = helpStyle.Render(helpText)
	}

	var content string
	if showSummaryPanel {
		summaryTitle := sectionTitleStyle.Render("Pending Changes")

		if len(m.PendingChanges()) > 0 {
			buttons := m.renderSummaryButtons()
			titleContentWidth := innerWidth
			gap := titleContentWidth - lipgloss.Width(summaryTitle) - lipgloss.Width(buttons)
			if gap < 1 {
				gap = 1
			}
			summaryTitle = summaryTitle + strings.Repeat(" ", gap) + buttons
		}

		var summaryFull string
		if m.changesApplied {
			summaryFull = contentStyle.Render(m.renderSavedSummary())
		} else {
			summaryBody := m.renderSummary()
			parts := []string{summaryTitle}
			if m.saveErr != nil {
				errStyle := lipgloss.NewStyle().Foreground(colorError).PaddingLeft(2).Width(innerWidth)
				parts = append(parts, "", errStyle.Render(m.saveErr.Error()))
			}
			parts = append(parts, "", contentStyle.Render(summaryBody))
			summaryFull = lipgloss.JoinVertical(lipgloss.Left, parts...)
		}

		var summarySection string
		if m.focus == FocusSummary {
			summarySection = focusedBorderStyle.Render(summaryFull)
		} else {
			summarySection = unfocusedBorderStyle.Render(summaryFull)
		}

		content = lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			mainSection,
			summarySection,
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

func (m Model) renderSummaryButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSubtle).
		Foreground(colorText)
	activeStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	if m.confirmingDiscard {
		promptStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
		prompt := promptStyle.Render("Discard all changes?")

		var yesBtn, noBtn string
		if m.discardConfirmIdx == 0 {
			yesBtn = activeStyle.Render("Yes")
		} else {
			yesBtn = buttonStyle.Render("Yes")
		}
		if m.discardConfirmIdx == 1 {
			noBtn = activeStyle.Render("No")
		} else {
			noBtn = buttonStyle.Render("No")
		}
		return prompt + "  " + lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, " ", noBtn)
	}

	focused := m.focus == FocusSummary && m.summaryOnButtons

	labels := []string{"Save", "Discard"}
	var parts []string
	for i, label := range labels {
		if focused && i == m.summaryButtonIdx {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, buttonStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

// renderHeader renders the header with breadcrumbs and optional environment label.
func (m Model) renderHeader(context string) string {
	slashStyle := lipgloss.NewStyle().
		Foreground(colorText)

	contextStyle := lipgloss.NewStyle().
		Foreground(colorText)

	terramateStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary)

	left := terramateStyle.Render("terramate")
	if context != "" {
		left += slashStyle.Render(" / ") +
			contextStyle.Render(context)
	}

	if m.viewState == ViewCloudLogin || m.viewState == ViewEnvSelect || m.selectedEnv == nil {
		return left
	}

	envStyle := lipgloss.NewStyle().Foreground(colorText)
	envLabel := envStyle.Render(m.selectedEnv.Name)

	gap := uiWidth + 2 - lipgloss.Width(left) - lipgloss.Width(envLabel)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + envLabel
}

// commandMeta maps each command name to its color and hint text.
var commandMeta = map[string]struct {
	color lipgloss.AdaptiveColor
	hint  string
}{
	"Create":      {color: colorCreate, hint: "Create infrastructure in your project"},
	"Reconfigure": {color: colorReconfig, hint: "Reconfigure existing infrastructure"},
	"Promote":     {color: colorPromote, hint: "Promote infrastructure to this environment"},
	"Upgrade":     {color: colorAccent, hint: "Upgrade a bundle to a newer version"},
	"Destroy":     {color: colorError, hint: "Remove a bundle from your project"},
	"Quit":        {color: colorTextMuted, hint: "Quit Terramate"},
}

func (m Model) renderCommandGrid() string {
	innerWidth := uiWidth - 4

	if m.confirmingExit {
		promptStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
		buttonStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorTextMuted)
		activeStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorPrimary).
			Foreground(lipgloss.Color("#000000")).
			Bold(true)

		prompt := promptStyle.Render("You still have unsaved changes. Exit anyway?")

		var yesBtn, noBtn string
		if m.exitConfirmIdx == 0 {
			yesBtn = activeStyle.Render("Yes")
		} else {
			yesBtn = buttonStyle.Render("Yes")
		}
		if m.exitConfirmIdx == 1 {
			noBtn = activeStyle.Render("No")
		} else {
			noBtn = buttonStyle.Render("No")
		}

		line := prompt + "  " + lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, " ", noBtn)
		gap := innerWidth - lipgloss.Width(line)
		if gap < 0 {
			gap = 0
		}
		return strings.Repeat(" ", gap) + line
	}

	indent := 4

	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)

	cmds := m.commands
	if len(cmds) == 0 {
		return ""
	}

	totalTextWidth := 0
	for _, cmd := range cmds {
		totalTextWidth += len(cmd)
	}

	usableWidth := innerWidth - indent
	totalButtonText := len(cmds)*indent + totalTextWidth
	totalGap := usableWidth - totalButtonText
	if totalGap < 0 {
		totalGap = 0
	}
	numGaps := len(cmds) - 1
	baseGap := 0
	extraGaps := 0
	if numGaps > 0 {
		baseGap = totalGap / numGaps
		extraGaps = totalGap % numGaps
	}

	var rowStr string
	for i, cmd := range cmds {
		isSelected := m.focus == FocusCommands && i == m.commandIdx

		var rendered string
		if isSelected {
			indicator := "›"
			if cm, ok := commandMeta[cmd]; ok {
				indicator = lipgloss.NewStyle().Foreground(cm.color).Render("›")
			}
			rendered = fmt.Sprintf("  %s %s", indicator, selectedStyle.Render(cmd))
		} else {
			rendered = fmt.Sprintf("    %s", normalStyle.Render(cmd))
		}

		if i > 0 {
			g := baseGap
			if i-1 < extraGaps {
				g++
			}
			rowStr += strings.Repeat(" ", g)
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

func (m Model) renderSummary() string {
	if len(m.PendingChanges()) == 0 {
		dimStyle := lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Italic(true)
		return dimStyle.Render("No pending changes")
	}

	iconStyle := map[ChangeKind]lipgloss.Style{
		ChangeCreate:   lipgloss.NewStyle().Foreground(colorCreate),
		ChangeReconfig: lipgloss.NewStyle().Foreground(colorReconfig),
		ChangePromote:  lipgloss.NewStyle().Foreground(colorPromote),
	}
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	nameStyle := lipgloss.NewStyle().Foreground(colorText)
	selectedNameStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	envStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	maxBundle, maxDisplay, maxEnv := 0, 0, 0
	for _, c := range m.PendingChanges() {
		if len(c.Metadata.Name) > maxBundle {
			maxBundle = len(c.Metadata.Name)
		}
		if len(c.DisplayName) > maxDisplay {
			maxDisplay = len(c.DisplayName)
		}

		envLen := 0
		if c.Env != nil {
			envLen += len(c.Env.ID)
		}
		if c.FromEnv != nil && c.FromEnv != c.Env {
			envLen += len(c.FromEnv.ID)
		}
		if envLen > maxEnv {
			maxEnv = envLen
		}
	}

	focused := m.focus == FocusSummary

	var rows []string
	for i, change := range m.PendingChanges() {
		icon := iconStyle[change.Kind].Render("◆")
		isSelected := focused && !m.summaryOnButtons && i == m.summaryCursor

		bundleName := change.Metadata.Name
		bundleCol := dimStyle.Render(bundleName + strings.Repeat(" ", maxBundle-len(bundleName)))

		display := change.DisplayName
		var displayCol string
		if isSelected {
			displayCol = selectedNameStyle.Render(display + strings.Repeat(" ", maxDisplay-len(display)))
		} else {
			displayCol = nameStyle.Render(display + strings.Repeat(" ", maxDisplay-len(display)))
		}

		envCol := ""
		if maxEnv > 0 {
			if change.Env != nil && change.FromEnv != nil && change.FromEnv != change.Env {
				envLen := len(change.Env.ID) + len(change.FromEnv.ID)
				envCol = envStyle.Render("["+change.Env.ID+" ← "+change.FromEnv.ID+"]") + strings.Repeat(" ", maxEnv-envLen)
			} else if change.Env != nil {
				envCol = envStyle.Render("["+change.Env.ID+"]") + strings.Repeat(" ", maxEnv-len(change.Env.ID))
			} else {
				envCol = strings.Repeat(" ", maxEnv+2)
			}
			envCol += "  "
		}

		var prefix string
		if isSelected {
			prefix = "› "
		} else {
			prefix = "  "
		}

		row := fmt.Sprintf("%s%s  %s  %s  %s", prefix, icon, bundleCol, displayCol, envCol)
		if change.Source != "" {
			row += dimStyle.Render("from " + change.Source)
		}
		row = truncateStyledRow(row, uiWidth-4)
		rows = append(rows, row)

		if len(change.Warnings) > 0 {
			warnStyle := lipgloss.NewStyle().Foreground(colorWarning)
			for _, w := range change.Warnings {
				rows = append(rows, "     "+warnStyle.Render("⚠ "+w))
			}
		}
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderSavedSummary() string {
	actionStyle := lipgloss.NewStyle().Foreground(colorText)
	nameStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	innerWidth := uiWidth - 8

	type fileLine struct {
		action string
		detail string
		path   string
	}

	var lines []fileLine
	for _, c := range m.savedChanges {
		var action, detail string
		switch c.Kind {
		case ChangeCreate:
			action = "Created"
			if c.EnvID != "" {
				detail = fmt.Sprintf("%s [%s]", c.Name, c.EnvID)
			} else {
				detail = c.Name
			}
		case ChangeReconfig:
			action = "Reconfigured"
			if c.EnvID != "" {
				detail = fmt.Sprintf("%s [%s]", c.Name, c.EnvID)
			} else {
				detail = c.Name
			}
		case ChangePromote:
			action = "Promoted"
			detail = fmt.Sprintf("%s from [%s] to [%s]", c.Name, c.FromEnvID, c.EnvID)
		default:
			panic("unsupported change kind " + c.Kind)
		}
		lines = append(lines, fileLine{action: action, detail: detail, path: c.ProjectPath})
	}

	maxAction := 0
	maxDetail := 0
	for _, l := range lines {
		if len(l.action) > maxAction {
			maxAction = len(l.action)
		}
		if len(l.detail) > maxDetail {
			maxDetail = len(l.detail)
		}
	}

	var rows []string
	for _, l := range lines {
		actionPad := strings.Repeat(" ", maxAction-len(l.action))
		namePad := strings.Repeat(" ", maxDetail-len(l.detail))
		row := fmt.Sprintf("  ✅ %s%s %s %s%s %s %s",
			actionStyle.Render(l.action), actionPad,
			nameStyle.Render("bundle"),
			nameStyle.Render(l.detail), namePad,
			dimStyle.Render("at"),
			dimStyle.Render(l.path),
		)
		row = truncateStyledRow(row, innerWidth)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// truncateStyledRow truncates a styled string to maxWidth visible characters,
// appending "…" if truncated. It walks the string rune-by-rune, tracking visible
// width while preserving ANSI escape sequences.
func truncateStyledRow(s string, maxWidth int) string {
	if maxWidth < 2 {
		maxWidth = 2
	}
	target := maxWidth - 1 // reserve 1 char for "…"
	var result []byte
	visibleWidth := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// ANSI escape sequence — copy it entirely without counting width.
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && s[j] != 'm' {
					j++
				}
				if j < len(s) {
					j++ // include 'm'
				}
			}
			result = append(result, s[i:j]...)
			i = j
			continue
		}
		if visibleWidth >= target {
			break
		}
		result = append(result, s[i])
		visibleWidth++
		i++
	}
	if visibleWidth >= target && i < len(s) {
		// Append ellipsis and reset to avoid style bleeding.
		result = append(result, "\x1b[0m…"...)
	}
	return string(result)
}
