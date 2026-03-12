// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/typeschema"
)

func (m Model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.chatProposalFocus {
		return m.updateChatProposalMode(msg)
	}
	return m.updateChatTypingMode(msg)
}

func (m Model) updateChatTypingMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		userText := strings.TrimSpace(m.promptInput.Value())
		if userText == "" || m.chatThinking {
			return m, nil
		}
		m.chatMessages = append(m.chatMessages, ChatMessage{
			Role:    "user",
			Content: userText,
		})
		m.promptInput.SetValue("")
		m.promptInput.SetHeight(1)
		m.chatThinking = true
		m.updateChatViewport()
		history := m.buildChatHistory()
		pendingProposals := m.pendingProposalIDs()
		bundleDefs, bundleInstances := m.buildBundleContext()
		cloudBaseURL := m.EngineState.CloudBaseURL
		cloudAuth := &m.EngineState.cloudAuth
		cliCfg := m.EngineState.CLIConfig
		llm := m.llmConfig
		return m, func() tea.Msg {
			resp, err := sendChatMessage(cloudBaseURL, cloudAuth, cliCfg, llm, userText, history, pendingProposals, bundleDefs, bundleInstances)
			return chatResponseMsg{resp: resp, err: err}
		}

	case "tab":
		if len(m.ProposedChanges()) > 0 {
			m.chatProposalFocus = true
			m.chatProposalCursor = 0
			m.promptInput.Blur()
			m.updateChatViewport()
		}
		return m, nil

	case "esc":
		if m.promptInput.Value() != "" {
			m.promptInput.SetValue("")
			m.promptInput.SetHeight(1)
			return m, nil
		}
		m.chatThinking = false
		m.chatProposalFocus = false
		m.chatProposalCursor = 0
		m.promptInput.SetValue("")
		m.promptInput.SetHeight(1)
		m.promptInput.Placeholder = "Describe what you want to do ... or select an action from ↓"
		m.viewState = ViewOverview
		m.focus = FocusPrompt
		m.promptInput.Focus()
		return m, textarea.Blink

	case "up":
		m.chatViewport.ScrollUp(1)
		return m, nil
	case "down":
		m.chatViewport.ScrollDown(1)
		return m, nil
	case "pgup":
		m.chatViewport.HalfPageUp()
		return m, nil
	case "pgdown":
		m.chatViewport.HalfPageDown()
		return m, nil

	default:
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		return m, cmd
	}
}

func (m Model) updateChatProposalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.ProposedChanges()) == 0 {
		m.chatProposalFocus = false
		m.proposalOnButtons = false
		m.promptInput.Focus()
		return m, textarea.Blink
	}

	switch msg.String() {
	case "tab", "esc":
		m.chatProposalFocus = false
		m.proposalOnButtons = false
		m.promptInput.Focus()
		m.updateChatViewport()
		return m, textarea.Blink

	case "up":
		if m.proposalOnButtons {
			m.proposalOnButtons = false
			m.chatProposalCursor = len(m.ProposedChanges()) - 1
			m.proposalButtonIdx = 0
		} else if m.chatProposalCursor > 0 {
			m.chatProposalCursor--
			m.proposalButtonIdx = 0
		} else {
			m.proposalOnButtons = true
			m.proposalTitleBtnIdx = 0
		}
		m.updateChatViewport()
		return m, nil

	case "down":
		if m.proposalOnButtons {
			m.proposalOnButtons = false
			m.chatProposalCursor = 0
			m.proposalButtonIdx = 0
		} else if m.chatProposalCursor < len(m.ProposedChanges())-1 {
			m.chatProposalCursor++
			m.proposalButtonIdx = 0
		} else {
			m.proposalOnButtons = true
			m.proposalTitleBtnIdx = 0
		}
		m.updateChatViewport()
		return m, nil

	case "left":
		if m.proposalOnButtons {
			if m.proposalTitleBtnIdx > 0 {
				m.proposalTitleBtnIdx--
			}
		} else {
			if m.proposalButtonIdx > 0 {
				m.proposalButtonIdx--
			}
		}
		return m, nil

	case "right":
		if m.proposalOnButtons {
			if m.proposalTitleBtnIdx < 1 {
				m.proposalTitleBtnIdx++
			}
		} else {
			if m.proposalButtonIdx < 2 {
				m.proposalButtonIdx++
			}
		}
		return m, nil

	case "enter":
		if m.proposalOnButtons {
			return m.executeProposalTitleButton()
		}
		idx := m.chatProposalCursor
		switch m.proposalButtonIdx {
		case 0: // Review
			if err := m.openReviewProposal(idx); err != nil {
				m.currentErr = err
			}
		case 1: // Accept
			pendingChanges := m.PendingChanges()
			proposedChanges := m.ProposedChanges()
			pendingChanges = append(pendingChanges, proposedChanges[idx])
			proposedChanges = append(proposedChanges[:idx], proposedChanges[idx+1:]...)

			m.SetPendingChanges(pendingChanges)
			m.SetProposedChanges(proposedChanges)
			m.changesApplied = false
			m.proposalButtonIdx = 0
			m.advanceProposalCursor()
			m.updateChatViewport()
		case 2: // Reject
			m.SetProposedChanges(append(m.ProposedChanges()[:idx], m.ProposedChanges()[idx+1:]...))
			m.proposalButtonIdx = 0
			m.advanceProposalCursor()
			m.updateChatViewport()
		}
		if !m.chatProposalFocus {
			return m, textarea.Blink
		}
		return m, nil
	}

	return m, nil
}

func (m Model) executeProposalTitleButton() (tea.Model, tea.Cmd) {
	switch m.proposalTitleBtnIdx {
	case 0: // Accept All
		m.SetPendingChanges(append(m.PendingChanges(), m.ProposedChanges()...))
		m.SetProposedChanges(nil)
		m.changesApplied = false
	case 1: // Reject All
		m.SetProposedChanges(nil)
	}
	m.proposalButtonIdx = 0
	m.proposalOnButtons = false
	m.advanceProposalCursor()
	m.updateChatViewport()
	if !m.chatProposalFocus {
		return m, textarea.Blink
	}
	return m, nil
}

// advanceProposalCursor moves the cursor to the next pending proposal,
// or exits proposal mode if none remain.
func (m *Model) advanceProposalCursor() {
	if len(m.ProposedChanges()) == 0 {
		m.chatProposalFocus = false
		m.proposalOnButtons = false
		m.chatProposalCursor = 0
		m.promptInput.Focus()
		return
	}
	if m.chatProposalCursor >= len(m.ProposedChanges()) {
		m.chatProposalCursor = len(m.ProposedChanges()) - 1
	}
}

// chatEffectiveContentHeight returns the content height available for the chat
// view, derived from the actual terminal height when known.
// Fixed overhead: outer padding (2) + title (1) + border (2) + inner padding (2)
// + help (1) = 8 lines. The separator line is already subtracted inside the
// vpHeight formula, so it is not counted here.
func (m Model) chatEffectiveContentHeight() int {
	if m.height > 0 {
		h := m.height - 8
		if h < uiContentHeight {
			h = uiContentHeight
		}
		return h
	}
	return uiContentHeight
}

// updateChatViewport rebuilds and scrolls the chat viewport content.
func (m *Model) updateChatViewport() {
	m.chatViewport.Width = m.effectiveWidth() - 4 - 2

	promptView := lipgloss.JoinHorizontal(lipgloss.Top, " ✨ ", m.promptInput.View())
	promptHeight := lipgloss.Height(promptView)

	proposalLines := len(m.ProposedChanges())
	proposalsPanelHeight := 0
	if proposalLines > 0 {
		// border(2) + padding(2) + title(1) + blank(1) + rows
		proposalsPanelHeight = 2 + 2 + 1 + 1 + proposalLines
	}

	vpHeight := m.chatEffectiveContentHeight() - promptHeight - 1 - proposalsPanelHeight
	if vpHeight < 4 {
		vpHeight = 4
	}
	m.chatViewport.Height = vpHeight

	m.chatViewport.SetContent(m.renderChatLog())
	m.chatViewport.GotoBottom()
}

func (m Model) renderChatView() string {
	chatBorderColor := colorBorderFocus
	if m.chatProposalFocus {
		chatBorderColor = colorBorder
	}
	panelWidth := m.effectiveWidth()

	chatBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(chatBorderColor).
		Padding(1, 2).
		Width(panelWidth)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	title := m.renderHeader("prompt", panelWidth)

	promptView := lipgloss.JoinHorizontal(lipgloss.Top, " ✨ ", m.promptInput.View())
	promptHeight := lipgloss.Height(promptView)

	panelInnerWidth := panelWidth - 4

	proposalsPanelHeight := 0
	var proposalsPanel string
	if len(m.ProposedChanges()) > 0 {
		proposalsBody := m.renderChatProposalsPanel(panelInnerWidth)
		proposalsBorder := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Width(panelWidth)
		if m.chatProposalFocus {
			proposalsBorder = proposalsBorder.BorderForeground(colorBorderFocus)
		}
		proposalsPanel = proposalsBorder.Render(proposalsBody)
		proposalsPanelHeight = lipgloss.Height(proposalsPanel)
	}

	separatorLine := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", panelInnerWidth))
	vpHeight := m.chatEffectiveContentHeight() - promptHeight - 1 - proposalsPanelHeight
	if vpHeight < 4 {
		vpHeight = 4
	}

	scrollbarGutter := 2
	m.chatViewport.Width = panelInnerWidth - scrollbarGutter
	m.chatViewport.Height = vpHeight

	maxOff := m.chatViewport.TotalLineCount() - m.chatViewport.Height
	if maxOff < 0 {
		maxOff = 0
	}
	if m.chatViewport.YOffset > maxOff {
		m.chatViewport.YOffset = maxOff
	}

	vpView := m.chatViewport.View()
	vpRenderedHeight := lipgloss.Height(vpView)
	scrollbar := m.renderChatScrollbar(vpRenderedHeight)
	if scrollbar != "" {
		vpView = lipgloss.JoinHorizontal(lipgloss.Top, vpView, " ", scrollbar)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		vpView,
		separatorLine,
		promptView,
	)

	var helpText string
	if m.chatProposalFocus {
		helpText = "←/→: switch action • enter: confirm • A: accept all • X: reject all • tab: back to prompt"
	} else {
		base := "enter: send • esc: return"
		if len(m.ProposedChanges()) > 0 {
			base = "enter: send • tab: review • esc: return"
		}
		helpText = base
	}
	help := helpStyle.Render(m.finalHelpText(helpText))

	parts := []string{title, chatBorderStyle.Render(inner)}
	if proposalsPanel != "" {
		parts = append(parts, proposalsPanel)
	}
	parts = append(parts, help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

// renderChatProposalsPanel renders the review panel content (title + buttons + list),
// styled like the overview's Pending Changes panel. Only pending proposals are shown.
func (m Model) renderChatProposalsPanel(innerWidth int) string {
	if len(m.ProposedChanges()) == 0 {
		return ""
	}

	sectionTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	contentStyle := lipgloss.NewStyle().Width(innerWidth)

	titleText := sectionTitleStyle.Render("Proposed Changes")
	buttons := m.renderProposalButtons()
	gap := innerWidth - lipgloss.Width(titleText) - lipgloss.Width(buttons)
	if gap < 1 {
		gap = 1
	}
	titleText = titleText + strings.Repeat(" ", gap) + buttons

	listBody := m.renderProposalsList(innerWidth)

	return lipgloss.JoinVertical(lipgloss.Left,
		titleText,
		"",
		contentStyle.Render(listBody),
	)
}

func (m Model) renderProposalButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSubtle).
		Foreground(colorText)
	activeStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	focused := m.chatProposalFocus && m.proposalOnButtons
	labels := []string{"Accept All", "Reject All"}
	var parts []string
	for i, label := range labels {
		if focused && i == m.proposalTitleBtnIdx {
			parts = append(parts, activeStyle.Render(label))
		} else {
			parts = append(parts, buttonStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

// renderProposalsList renders pending proposal rows matching the overview's pending changes style.
func (m Model) renderProposalsList(innerWidth int) string {
	iconStyle := map[ChangeKind]lipgloss.Style{
		ChangeCreate:   lipgloss.NewStyle().Foreground(colorSuccess),
		ChangeReconfig: lipgloss.NewStyle().Foreground(colorWarning),
	}
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	nameStyle := lipgloss.NewStyle().Foreground(colorText)
	selectedNameStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	buttonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSubtle).
		Foreground(colorText)
	activeButtonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	labels := []string{"Review", "Accept", "Reject"}
	var sampleBtns []string
	for _, label := range labels {
		sampleBtns = append(sampleBtns, buttonStyle.Render(label))
	}
	buttonsStr := "  " + strings.Join(sampleBtns, " ")
	buttonsWidth := lipgloss.Width(buttonsStr)

	focusIdx := -1
	if m.chatProposalFocus && !m.proposalOnButtons && m.chatProposalCursor < len(m.ProposedChanges()) {
		focusIdx = m.chatProposalCursor
	}

	maxBundle, maxDisplay := 0, 0
	for _, c := range m.ProposedChanges() {
		if len(c.Metadata.Name) > maxBundle {
			maxBundle = len(c.Metadata.Name)
		}
		if len(c.DisplayName) > maxDisplay {
			maxDisplay = len(c.DisplayName)
		}
	}

	var rows []string
	for i, c := range m.ProposedChanges() {
		isFocused := i == focusIdx

		icon := iconStyle[c.Kind].Render("◆")

		proposalTag := dimStyle.Render(fmt.Sprintf("#%d", c.ProposalID))
		bundleName := c.Metadata.Name
		display := c.DisplayName

		var prefix string
		if isFocused {
			prefix = "› "
		} else {
			prefix = "  "
		}

		bundleCol := dimStyle.Render(bundleName + strings.Repeat(" ", maxBundle-len(bundleName)))
		var displayCol string
		if isFocused {
			displayCol = selectedNameStyle.Render(display + strings.Repeat(" ", maxDisplay-len(display)))
		} else {
			displayCol = nameStyle.Render(display + strings.Repeat(" ", maxDisplay-len(display)))
		}

		row := fmt.Sprintf("%s%s %s  %s  %s", prefix, icon, proposalTag, bundleCol, displayCol)
		if c.Source != "" {
			row += "  " + dimStyle.Render("from "+c.Source)
		}

		if isFocused {
			maxRowWidth := innerWidth - buttonsWidth
			rowWidth := lipgloss.Width(row)
			if rowWidth > maxRowWidth {
				row = truncateStyledRow(row, maxRowWidth)
				rowWidth = lipgloss.Width(row)
			}

			var btns []string
			for j, label := range labels {
				if j == m.proposalButtonIdx {
					btns = append(btns, activeButtonStyle.Render(label))
				} else {
					btns = append(btns, buttonStyle.Render(label))
				}
			}
			btnsPart := strings.Join(btns, " ")
			gap := innerWidth - rowWidth - lipgloss.Width(btnsPart)
			if gap < 2 {
				gap = 2
			}
			row += strings.Repeat(" ", gap) + btnsPart
		} else {
			row = truncateStyledRow(row, innerWidth)
		}

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

func (m Model) renderChatScrollbar(trackHeight int) string {
	totalLines := m.chatViewport.TotalLineCount()
	visibleLines := m.chatViewport.VisibleLineCount()

	if totalLines <= visibleLines {
		return ""
	}

	if trackHeight < 1 {
		trackHeight = 1
	}

	thumbSize := (visibleLines * trackHeight) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}

	scrollFraction := float64(m.chatViewport.YOffset) / float64(totalLines-visibleLines)
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

// ensureChatRenderer returns a cached glamour renderer, creating or
// recreating it only when the viewport width changes.
func (m *Model) ensureChatRenderer() *glamour.TermRenderer {
	w := m.chatViewport.Width
	if m.chatRenderer != nil && m.chatRendererWidth == w {
		return m.chatRenderer
	}
	// The "dark" style applies a document margin of 2, which is added after
	// word-wrap. Subtract it so the rendered output fits the viewport width.
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(w-2),
	)
	m.chatRenderer = r
	m.chatRendererWidth = w
	// Invalidate per-message caches since word-wrap width changed.
	for i := range m.chatMessages {
		m.chatMessages[i].rendered = ""
		m.chatMessages[i].renderedWidth = 0
	}
	return r
}

func (m *Model) renderChatLog() string {
	innerWidth := m.chatViewport.Width
	dimStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	renderer := m.ensureChatRenderer()

	userBubbleStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(innerWidth - 2)

	localOnlyStyle := lipgloss.NewStyle().
		Foreground(colorSuccess).
		Italic(true)

	var sections []string
	for i := range m.chatMessages {
		msg := &m.chatMessages[i]
		if msg.Hidden {
			continue
		}
		if msg.LocalOnly {
			var lines []string
			for _, line := range strings.Split(msg.Content, "\n") {
				if line != "" {
					lines = append(lines, "  "+localOnlyStyle.Render("▸ "+line))
				}
			}
			sections = append(sections, strings.Join(lines, "\n"))
			sections = append(sections, "")
			continue
		}
		if msg.Role == "user" {
			sections = append(sections, userBubbleStyle.Render(msg.Content))
		} else {
			if msg.rendered != "" && msg.renderedWidth == innerWidth {
				sections = append(sections, msg.rendered)
			} else {
				rendered := msg.Content
				if renderer != nil && rendered != "" {
					if md, err := renderer.Render(rendered); err == nil {
						rendered = strings.TrimSpace(md)
					}
				}
				msg.rendered = rendered
				msg.renderedWidth = innerWidth
				sections = append(sections, rendered)
			}
		}
		sections = append(sections, "")
	}

	if m.chatThinking {
		thinkingStyle := lipgloss.NewStyle().Foreground(colorTextMuted).Italic(true)
		sections = append(sections, "  "+thinkingStyle.Render("Thinking..."))
	}

	if len(sections) == 0 {
		return dimStyle.Render("  Start a conversation...")
	}

	return strings.Join(sections, "\n")
}

// openReviewProposal sets up the edit form for reviewing an AI-proposed change.
// For AI-proposed changes that lack InputDefs, it resolves the bundle from
// localBundleDefs by matching the DisplayName and loads definitions on the fly.
func (m *Model) openReviewProposal(idx int) error {
	m.reviewingChange = idx
	change := m.ProposedChanges()[idx]

	est := m.EngineState

	evalctx := newBundleEvalContext(est.Evalctx, m.EngineState.Registry.Registry, m.selectedEnv)

	bde := m.findBundleDefByName(change.Metadata.Name)
	if bde == nil {
		return errors.E("review not available: bundle %q not found in local definitions", change.Metadata.Name)
	}

	schemas, err := config.EvalBundleSchemaNamespaces(est.Root, est.ResolveAPI, evalctx, bde.Define, true)
	if err != nil {
		return errors.E(err, "failed to load bundle schema")
	}

	schemactx := typeschema.EvalContext{
		Evalctx: evalctx,
		Schemas: schemas,
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errors.E(err, "failed to evaluate input definitions")
	}

	if change.Kind != ChangeReconfig {
		if bde.Define.Scaffolding.Name == nil {
			inputDefs = append(inputDefs, pseudoStringInput(
				pseudoKeyOutputName, "Instance name",
				"Name of the created bundle instance.",
			))
		}
		if bde.Define.Scaffolding.Path == nil {
			inputDefs = append(inputDefs, pseudoStringInput(
				pseudoKeyOutputPath, "Output file",
				"Path of the created code file.\nPaths starting with / are relative to the project root.\nOtherwise, they are relative to the current directory.",
			))
		}
	}

	m.ProposedChanges()[idx].InputDefs = inputDefs
	m.ProposedChanges()[idx].Source = bde.Tree.Dir().String()
	m.ProposedChanges()[idx].Metadata = *bde.Metadata
	m.selectedBundleDefEntry = bde

	m.inputsForm = NewInputsFormWithValues(
		inputDefs, schemactx, est.Registry, m.selectedEnv, nil, change.Values, change.OriginalValues,
	)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.confirmLabel = "Accept"
	m.inputsForm.cancelLabel = "Reject"
	m.inputsForm.skipDiscardConfirm = true
	m.inputsForm.focus = InputFocusActive
	m.viewState = ViewReviewProposal
	return nil
}

// findBundleDefByName looks up a bundle definition entry by metadata name.
func (m *Model) findBundleDefByName(name string) *config.BundleDefinitionEntry {
	est := m.EngineState
	for i := range est.LocalBundleDefs {
		if est.LocalBundleDefs[i].Metadata.Name == name {
			return &est.LocalBundleDefs[i]
		}
	}
	return nil
}

// findBundleDefBySource looks up a bundle definition entry by its source directory path.
func (m *Model) findLocalBundleDefBySource(source string) *config.BundleDefinitionEntry {
	est := m.EngineState
	for i := range est.LocalBundleDefs {
		if est.LocalBundleDefs[i].Source == source {
			return &est.LocalBundleDefs[i]
		}
	}
	return nil
}

func (m Model) updateReviewProposal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Escape) && !m.inputsForm.IsMultilineActive() {
		m.viewState = ViewChat
		m.chatProposalFocus = true
		m.promptInput.Blur()
		m.updateChatViewport()
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	idx := m.reviewingChange
	switch m.inputsForm.State() {
	case InputsFormAccepted:
		change := m.ProposedChanges()[idx]
		change.Values = m.inputsForm.Values()
		m.SetPendingChanges(append(m.PendingChanges(), change))
		m.SetProposedChanges(append(m.ProposedChanges()[:idx], m.ProposedChanges()[idx+1:]...))
		m.changesApplied = false
		m.advanceProposalCursor()
		m.viewState = ViewChat
		m.promptInput.Focus()
		m.updateChatViewport()
		return m, textarea.Blink
	case InputsFormDiscarded:
		m.SetProposedChanges(append(m.ProposedChanges()[:idx], m.ProposedChanges()[idx+1:]...))
		m.advanceProposalCursor()
		m.viewState = ViewChat
		m.promptInput.Focus()
		m.updateChatViewport()
		return m, textarea.Blink
	}

	return m, cmd
}

func (m Model) renderReviewProposalView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	change := m.ProposedChanges()[m.reviewingChange]
	headerContext := fmt.Sprintf("review / %s", change.Name)
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
