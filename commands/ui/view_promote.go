// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
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
		if m.promoteFilterPos >= 0 {
			m.promoteFilterPos = -1
			m.applyPromoteFilter()
			return m, nil
		}
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

	case msg.String() == "e":
		if len(m.promoteFilters) > 0 {
			m.promoteFilterPos = (m.promoteFilterPos + 1) % len(m.promoteFilters)
			m.applyPromoteFilter()
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if m.promoteCursor < len(m.promoteBundles) {
			targetEnv := m.promoteTargetEnvs[m.promoteCursor]
			if err := m.loadPromoteBundle(m.promoteBundles[m.promoteCursor], targetEnv); err != nil {
				return m.updateError(err)
			}
			m.viewState = ViewPromoteInput
			return m, nil
		}
	}
	return m, nil
}

// applyPromoteFilter rebuilds the bundle list based on the current filter position.
func (m *Model) applyPromoteFilter() {
	m.promoteBundles, m.promoteTargetEnvs = m.buildAllPromoteBundles()
	m.promoteCursor = 0
}

// loadPromoteBundle loads the bundle definition for the given bundle,
// evaluates its input definitions, and creates the inputs form pre-populated
// with the bundle's current values.
func (m *Model) loadPromoteBundle(b *config.Bundle, targetEnv *config.Environment) error {
	est := m.EngineState
	// We create a BundleDefinitionEntry
	bde := makeBundleDefinitionEntry(est.Root, b)

	schemactx, err := m.loadBundleEvalContext(bde, targetEnv)
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
	m.inputsForm = NewInputsFormWithValues(inputDefs, schemactx, est.Registry, targetEnv, b.Environment, values, values)
	m.inputsForm.confirmLabel = "Save"
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
	return nil
}

// currentPromoteFilter returns the current filter state, or nil if showing all.
func (m Model) currentPromoteFilter() *envFilterState {
	if m.promoteFilterPos >= 0 && m.promoteFilterPos < len(m.promoteFilters) {
		return &m.promoteFilters[m.promoteFilterPos]
	}
	return nil
}

// nextPromoteFilterName returns the short ID of the next filter in the cycle.
func (m Model) nextPromoteFilterName() string {
	if len(m.promoteFilters) == 0 {
		return ""
	}
	nextPos := (m.promoteFilterPos + 1) % len(m.promoteFilters)
	return m.promoteFilters[nextPos].shortID
}

// buildPromoteFilters precomputes the list of target envs that have promotable bundles.
func (m Model) buildPromoteFilters() []envFilterState {
	// Build all promotable bundles unfiltered to find which target envs have results
	targetEnvHas := make(map[string]bool)
	for _, targetEnv := range m.EngineState.Registry.Environments {
		if targetEnv.PromoteFrom == "" {
			continue
		}
		// Temporarily check if this target env has promotable bundles
		envAliases := make(map[string]bool)
		for _, b := range m.EngineState.Registry.Bundles {
			if b.Environment != nil && b.Environment.ID == targetEnv.ID {
				envAliases[b.Alias] = true
			}
		}
		for _, b := range m.EngineState.Registry.Bundles {
			if b.Environment == nil || b.Environment.ID != targetEnv.PromoteFrom {
				continue
			}
			if !envAliases[b.Alias] {
				targetEnvHas[targetEnv.ID] = true
				break
			}
		}
	}

	var states []envFilterState
	for _, env := range m.EngineState.Registry.Environments {
		if targetEnvHas[env.ID] {
			states = append(states, envFilterState{
				env:     env,
				label:   env.Name,
				shortID: env.ID,
			})
		}
	}
	return states
}

// buildAllPromoteBundles returns all promotable bundles across all environments.
// For each environment with PromoteFrom, finds bundles from the source env
// that don't already exist (by alias) in the target env.
// Returns parallel slices: bundles and their corresponding target environments.
func (m Model) buildAllPromoteBundles() ([]*config.Bundle, []*config.Environment) {
	est := m.EngineState

	// Build alias index per env: envID -> set of aliases
	envAliases := make(map[string]map[string]bool)
	for _, b := range est.Registry.Bundles {
		if b.Environment == nil {
			continue
		}
		if _, ok := envAliases[b.Environment.ID]; !ok {
			envAliases[b.Environment.ID] = make(map[string]bool)
		}
		envAliases[b.Environment.ID][b.Alias] = true
	}

	var bundles []*config.Bundle
	var targetEnvs []*config.Environment

	for _, targetEnv := range est.Registry.Environments {
		if targetEnv.PromoteFrom == "" {
			continue
		}

		// Apply env filter: only show bundles promotable into the filtered target env
		if f := m.currentPromoteFilter(); f != nil && f.env.ID != targetEnv.ID {
			continue
		}

		existing := envAliases[targetEnv.ID]
		for _, b := range est.Registry.Bundles {
			if b.Environment == nil || b.Environment.ID != targetEnv.PromoteFrom {
				continue
			}
			if existing[b.Alias] {
				continue
			}
			bundles = append(bundles, b)
			targetEnvs = append(targetEnvs, targetEnv)
		}
	}

	// Sort into grouped display order so the flat cursor index matches
	// the visual position. Keep targetEnvs in sync.
	groups := groupBundles(bundles)
	sorted := make([]*config.Bundle, 0, len(bundles))
	sortedEnvs := make([]*config.Environment, 0, len(bundles))
	for _, g := range groups {
		for _, idx := range g.offsets {
			sorted = append(sorted, bundles[idx])
			sortedEnvs = append(sortedEnvs, targetEnvs[idx])
		}
	}
	return sorted, sortedEnvs
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

	breadcrumb := "Promote Bundle Instance"
	if f := m.currentPromoteFilter(); f != nil {
		breadcrumb = "Promote Bundle Instance to " + f.label
	}
	title := m.renderHeader(breadcrumb)

	// Detail box for highlighted bundle
	var detailBox string
	if m.promoteCursor < len(m.promoteBundles) {
		b := m.promoteBundles[m.promoteCursor]
		fields := []detailField{
			{label: "Bundle", value: b.DefinitionMetadata.Name + " v" + b.DefinitionMetadata.Version, truncEnd: true},
		}
		if b.DefinitionMetadata.Class != "" {
			fields = append(fields, detailField{label: "Class", value: b.DefinitionMetadata.Class, truncEnd: true})
		}
		fields = append(fields, detailField{label: "Alias", value: displayNameFromAlias(b.Alias, b.Name), truncEnd: true})
		if m.promoteCursor < len(m.promoteTargetEnvs) {
			sourceEnvName := envNameForID(est.Registry.Environments, b.Environment.ID)
			targetEnvName := m.promoteTargetEnvs[m.promoteCursor].Name
			fields = append(fields, detailField{label: "Promote", value: sourceEnvName + " → " + targetEnvName, truncEnd: true})
		}
		fields = append(fields, detailField{}) // separator
		fields = append(fields, detailField{label: "Source", value: b.Source})
		hostPath := project.PrjAbsPath(est.Root.HostDir(), b.Info.HostPath()).String()
		if hostPath != "" {
			fields = append(fields, detailField{label: "Config", value: hostPath})
		}
		detailBox = renderDetailBox(innerWidth, "Bundle Instance Details", fields)
	}

	header := lipgloss.JoinVertical(lipgloss.Left, detailBox, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := m.effectiveContentHeight() - headerHeight

	groups := groupBundles(m.promoteBundles)
	selectedGroupIdx, items := m.renderPromoteGroupedItems(groups, m.promoteCursor, contentWidth)

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

	inner := contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, listContent))

	escLabel := "esc: back"
	if m.promoteFilterPos >= 0 {
		escLabel = "esc: reset filter"
	}
	helpParts := "↑↓: Select Bundle • " + escLabel
	if len(m.promoteFilters) > 0 {
		helpParts += " • e: show target env " + m.nextPromoteFilterName()
	}
	help := helpStyle.Render(m.finalHelpText(helpParts))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		borderStyle.Render(inner),
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

// renderPromoteGroupedItems renders grouped promote bundles as a flat list of renderedItems.
// Group headers are non-selectable separator items, instances are individual items.
// Returns the index of the selected item in the flat list, suitable for scrollWindowVar.
func (m Model) renderPromoteGroupedItems(groups []bundleGroup, cursor, contentWidth int) (int, []renderedItem) {
	est := m.EngineState

	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	headerNameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	versionStyle := lipgloss.NewStyle().Foreground(colorTextSubtle)
	envStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	lineStyle := lipgloss.NewStyle().Width(contentWidth)

	// First pass: find max alias width for alignment
	maxAliasWidth := 0
	for _, g := range groups {
		for _, b := range g.bundles {
			w := lipgloss.Width(displayNameFromAlias(b.Alias, b.Name))
			if w > maxAliasWidth {
				maxAliasWidth = w
			}
		}
	}

	var items []renderedItem
	selectedItemIdx := 0
	visualIdx := 0

	for gi, g := range groups {
		b0 := g.bundles[0]

		// Empty line before group (except first)
		if gi > 0 {
			items = append(items, renderedItem{content: "", height: 1})
		}

		// Group header: non-selectable
		headerLine := headerNameStyle.Render(g.name) + " " + versionStyle.Render("v"+b0.DefinitionMetadata.Version)
		items = append(items, renderedItem{content: lineStyle.Render(headerLine), height: 1})

		// Instance rows
		for i, b := range g.bundles {
			isSelected := visualIdx == cursor
			if isSelected {
				selectedItemIdx = len(items)
			}
			visualIdx++

			displayName := displayNameFromAlias(b.Alias, b.Name)
			pad := strings.Repeat(" ", maxAliasWidth-lipgloss.Width(displayName)+2)

			var line string
			if isSelected {
				line = selectedStyle.Render("  › "+displayName) + pad
			} else {
				line = "    " + displayName + pad
			}

			// Show env flow: source → target
			globalBundleIdx := g.offsets[i]
			if globalBundleIdx < len(m.promoteTargetEnvs) && b.Environment != nil {
				sourceEnvName := envNameForID(est.Registry.Environments, b.Environment.ID)
				targetEnvName := m.promoteTargetEnvs[globalBundleIdx].Name
				line += envStyle.Render(sourceEnvName + " → " + targetEnvName)
			}

			items = append(items, renderedItem{content: lineStyle.Render(line), height: 1})
		}
	}

	return selectedItemIdx, items
}

func (m Model) renderPromoteInputView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	b := m.promoteBundle
	aliasStyle := lipgloss.NewStyle().Foreground(colorCreate)
	alias := aliasStyle.Render(displayNameFromAlias(b.Alias, b.Name))
	var envTag string
	if m.promoteCursor < len(m.promoteTargetEnvs) {
		targetEnv := m.promoteTargetEnvs[m.promoteCursor]
		envStyle := lipgloss.NewStyle().Foreground(colorPromote)
		envTag = " " + envStyle.Render("["+targetEnv.Name+"]")
	}
	headerContext := "Promote " + b.DefinitionMetadata.Name + ": " + alias + envTag
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

	if key.Matches(msg, keys.Escape) && !m.inputsForm.IsMultilineActive() && !m.inputsForm.confirmingDiscard {
		m.viewState = ViewPromoteSelect
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		targetEnv := m.promoteTargetEnvs[m.promoteCursor]
		change, err := NewPromoteChange(
			est, targetEnv, m.promoteBundle, m.selectedBundleDefEntry,
			m.inputsForm.Schemactx, m.inputsForm.InputDefs, m.inputsForm.Values(),
		)
		if err != nil {
			m.inputsForm.SetValidationError(err)
			m.inputsForm.state = InputsFormActive
			break
		}
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

		m.viewState = ViewOverview
	case InputsFormDiscarded:
		m.viewState = ViewOverview
	}

	return m, cmd
}
