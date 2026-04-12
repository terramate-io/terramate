// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/typeschema"
)

func (m Model) updateReconfigSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		if m.reconfigFilterPos >= 0 {
			m.reconfigFilterPos = -1
			m.applyReconfigFilter()
			return m, nil
		}
		m.viewState = ViewOverview
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.reconfigCursor > 0 {
			m.reconfigCursor--
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.reconfigCursor < len(m.reconfigBundles)-1 {
			m.reconfigCursor++
		}
		return m, nil

	case msg.String() == "e":
		if len(m.reconfigFilters) > 0 {
			m.reconfigFilterPos = (m.reconfigFilterPos + 1) % len(m.reconfigFilters)
			m.applyReconfigFilter()
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if m.reconfigCursor < len(m.reconfigBundles) {
			if err := m.loadReconfigBundle(m.reconfigBundles[m.reconfigCursor]); err != nil {
				return m.updateError(err)
			}
			m.viewState = ViewReconfigInput
			return m, nil
		}
	}
	return m, nil
}

// applyReconfigFilter rebuilds the bundle list based on the current filter position.
func (m *Model) applyReconfigFilter() {
	m.reconfigBundles = m.buildReconfigBundles()
	m.reconfigCursor = 0
}

// loadReconfigBundle loads the bundle definition for the given bundle,
// evaluates its input definitions, and creates the inputs form pre-populated
// with the bundle's current values.
func (m *Model) loadReconfigBundle(b *config.Bundle) error {
	est := m.EngineState
	// We create a BundleDefinitionEntry
	bde := makeBundleDefinitionEntry(est.Root, b)

	schemactx, err := m.loadBundleEvalContext(bde, b.Environment)
	if err != nil {
		return err
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errors.E(err, "failed to evaluate input definitions")
	}

	values := inputsToValueMap(b.Inputs)

	m.reconfigBundle = b
	m.selectedBundleDefEntry = bde
	m.inputsForm = NewInputsFormWithValues(inputDefs, schemactx, est.Registry, b.Environment, nil, values, values)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
	return nil
}

// currentReconfigFilter returns the current filter state, or nil if showing all.
func (m Model) currentReconfigFilter() *envFilterState {
	if m.reconfigFilterPos >= 0 && m.reconfigFilterPos < len(m.reconfigFilters) {
		return &m.reconfigFilters[m.reconfigFilterPos]
	}
	return nil
}

// nextReconfigFilterName returns the short ID of the next filter in the cycle.
func (m Model) nextReconfigFilterName() string {
	if len(m.reconfigFilters) == 0 {
		return ""
	}
	nextPos := (m.reconfigFilterPos + 1) % len(m.reconfigFilters)
	return m.reconfigFilters[nextPos].shortID
}

// buildReconfigFilters precomputes the list of valid filter states
// (environments that have reconfigurable bundles, plus env-less if applicable).
func (m Model) buildReconfigFilters() []envFilterState {
	pending := make(map[string]bool, len(m.PendingChanges()))
	for _, c := range m.PendingChanges() {
		if c.Kind == ChangeReconfig {
			pending[c.HostPath] = true
		}
	}

	// Check which envs have bundles, and whether env-less bundles exist
	envHas := make(map[string]bool)
	hasEnvLess := false
	for _, b := range m.EngineState.Registry.Bundles {
		if pending[b.Info.HostPath()] {
			continue
		}
		if b.Environment == nil {
			hasEnvLess = true
		} else {
			envHas[b.Environment.ID] = true
		}
	}

	var states []envFilterState
	for _, env := range m.EngineState.Registry.Environments {
		if envHas[env.ID] {
			states = append(states, envFilterState{
				env:     env,
				label:   env.Name,
				shortID: env.ID,
			})
		}
	}
	if hasEnvLess {
		states = append(states, envFilterState{
			envLess: true,
			label:   "Without Environment",
			shortID: "env-less",
		})
	}
	return states
}

// loadBundleEvalContext creates a bundle eval context and loads the schema namespaces for the given bundle.
func (m Model) loadBundleEvalContext(bde *config.BundleDefinitionEntry, env *config.Environment) (typeschema.EvalContext, error) {
	est := m.EngineState
	evalctx := newBundleEvalContext(est.Evalctx, est.Registry.Registry, env)
	schemas, err := config.EvalBundleSchemaNamespaces(est.Root, est.ResolveAPI, evalctx, bde.Define, true)
	if err != nil {
		return typeschema.EvalContext{}, errors.E(err, "Failed to load bundle schema.")
	}
	return typeschema.EvalContext{
		Evalctx: evalctx,
		Schemas: schemas,
	}, nil
}

// makeBundleDefinitionEntry constructs a BundleDefinitionEntry from an existing, already loaded bundle.
func makeBundleDefinitionEntry(root *config.Root, b *config.Bundle) *config.BundleDefinitionEntry {
	// This cannot fail. If we have the evaluated config.Bundle already, the HCL define must exist.
	tree, _ := root.Lookup(b.ResolvedSource)
	for _, def := range tree.Node.Defines {
		if def.Bundle != nil {
			return &config.BundleDefinitionEntry{
				Tree:     tree,
				Metadata: &b.DefinitionMetadata,
				Define:   def.Bundle,
			}
		}
	}
	return nil
}

// buildReconfigBundles returns bundles that do not already have a pending
// ChangeReconfig entry, sorted into grouped display order so that
// the flat cursor index matches the visual position.
func (m Model) buildReconfigBundles() []*config.Bundle {
	pending := make(map[string]bool, len(m.PendingChanges()))
	for _, c := range m.PendingChanges() {
		if c.Kind == ChangeReconfig {
			pending[c.HostPath] = true
		}
	}

	var filtered []*config.Bundle
	for _, b := range m.EngineState.Registry.Bundles {
		if pending[b.Info.HostPath()] {
			continue
		}

		f := m.currentReconfigFilter()
		if f != nil {
			if f.envLess {
				if b.Environment != nil {
					continue
				}
			} else if b.Environment == nil || b.Environment.ID != f.env.ID {
				continue
			}
		}
		filtered = append(filtered, b)
	}

	groups := groupBundles(filtered)
	sorted := make([]*config.Bundle, 0, len(filtered))
	for _, g := range groups {
		sorted = append(sorted, g.bundles...)
	}
	return sorted
}

type bundleGroup struct {
	name    string
	detail  string
	bundles []*config.Bundle
	offsets []int // cursor positions in the flat list
}

// groupBundles groups bundles by definition identity (name + version + source),
// sorted alphabetically by group name, with instances sorted by alias within each group.
func groupBundles(bundles []*config.Bundle) []bundleGroup {
	var groups []bundleGroup
	seen := map[string]int{}

	for i, b := range bundles {
		key := b.DefinitionMetadata.Name + "\x00" + b.DefinitionMetadata.Version + "\x00" + b.Source
		if gIdx, ok := seen[key]; ok {
			groups[gIdx].bundles = append(groups[gIdx].bundles, b)
			groups[gIdx].offsets = append(groups[gIdx].offsets, i)
			continue
		}
		seen[key] = len(groups)
		groups = append(groups, bundleGroup{
			name:    b.DefinitionMetadata.Name,
			detail:  fmt.Sprintf("v%s from %s", b.DefinitionMetadata.Version, b.Source),
			bundles: []*config.Bundle{b},
			offsets: []int{i},
		})
	}

	// Sort groups deterministically by name, then detail (version+source) as tiebreaker
	slices.SortFunc(groups, func(a, b bundleGroup) int {
		if c := cmp.Compare(a.name, b.name); c != 0 {
			return c
		}
		return cmp.Compare(a.detail, b.detail)
	})

	// Sort instances within each group by alias
	for i := range groups {
		g := &groups[i]
		indices := make([]int, len(g.bundles))
		for j := range indices {
			indices[j] = j
		}
		slices.SortFunc(indices, func(a, b int) int {
			return cmp.Compare(g.bundles[a].Alias, g.bundles[b].Alias)
		})
		sortedBundles := make([]*config.Bundle, len(g.bundles))
		sortedOffsets := make([]int, len(g.offsets))
		for j, idx := range indices {
			sortedBundles[j] = g.bundles[idx]
			sortedOffsets[j] = g.offsets[idx]
		}
		g.bundles = sortedBundles
		g.offsets = sortedOffsets
	}

	return groups
}

// renderGroupedBundleItems renders grouped bundles as a flat list of renderedItems.
// Group headers are non-selectable separator items, instances are individual items.
// Returns the index of the selected item in the flat list, suitable for scrollWindowVar.
func (m Model) renderGroupedBundleItems(groups []bundleGroup, cursor, contentWidth int) (int, []renderedItem) {
	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	headerNameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	versionStyle := lipgloss.NewStyle().Foreground(colorTextSubtle)
	envStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	lineStyle := lipgloss.NewStyle().Width(contentWidth)

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
		for _, b := range g.bundles {
			isSelected := visualIdx == cursor
			if isSelected {
				selectedItemIdx = len(items)
			}
			visualIdx++

			displayName := displayNameFromAlias(b.Alias, b.Name)
			var line string
			if isSelected {
				line = selectedStyle.Render("  › " + displayName)
			} else {
				line = "    " + displayName
			}
			if b.Environment != nil {
				line += " " + envStyle.Render("["+b.Environment.Name+"]")
			}

			items = append(items, renderedItem{content: lineStyle.Render(line), height: 1})
		}
	}

	return selectedItemIdx, items
}

func (m Model) renderReconfigSelectView() string {
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

	breadcrumb := "Reconfigure Bundle Instance"
	if f := m.currentReconfigFilter(); f != nil {
		if f.envLess {
			breadcrumb = "Reconfigure Bundle Instance Without Environment"
		} else {
			breadcrumb = "Reconfigure Bundle Instance in " + f.label
		}
	}
	title := m.renderHeader(breadcrumb)

	// Detail box for highlighted bundle
	var detailBox string
	if m.reconfigCursor < len(m.reconfigBundles) {
		b := m.reconfigBundles[m.reconfigCursor]
		fields := []detailField{
			{label: "Bundle", value: b.DefinitionMetadata.Name + " v" + b.DefinitionMetadata.Version, truncEnd: true},
		}
		if b.DefinitionMetadata.Class != "" {
			fields = append(fields, detailField{label: "Class", value: b.DefinitionMetadata.Class, truncEnd: true})
		}
		fields = append(fields, detailField{label: "Alias", value: displayNameFromAlias(b.Alias, b.Name), truncEnd: true})
		envName := "n/a"
		if b.Environment != nil {
			envName = b.Environment.Name
		}
		fields = append(fields, detailField{label: "Environment", value: envName, truncEnd: true})
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

	groups := groupBundles(m.reconfigBundles)
	selectedGroupIdx, items := m.renderGroupedBundleItems(groups, m.reconfigCursor, contentWidth)

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
	if m.reconfigFilterPos >= 0 {
		escLabel = "esc: reset filter"
	}
	helpParts := "↑↓: Select Bundle • " + escLabel
	if len(m.reconfigFilters) > 0 {
		helpParts += " • e: show only " + m.nextReconfigFilterName()
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

func (m Model) renderReconfigInputView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	b := m.reconfigBundle
	headerContext := fmt.Sprintf("Reconfigure Bundle Instance / %s", b.Name)
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

func (m Model) updateReconfigInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState

	if key.Matches(msg, keys.Escape) && !m.inputsForm.IsMultilineActive() {
		m.viewState = ViewReconfigSelect
		return m, nil
	}

	var cmd tea.Cmd
	m.inputsForm, cmd = m.inputsForm.Update(msg)

	switch m.inputsForm.State() {
	case InputsFormAccepted:
		if m.inputsForm.HasPendingChanges() {
			change, err := NewReconfigChange(
				est, m.reconfigBundle, m.selectedBundleDefEntry,
				m.inputsForm.Schemactx, m.inputsForm.InputDefs, m.inputsForm.Values(),
			)
			if err != nil {
				m.inputsForm.SetValidationError(err)
				m.inputsForm.state = InputsFormActive
				break
			}
			m.SetPendingChanges(append(m.PendingChanges(), change))
			m.changesApplied = false
		}
		m.viewState = ViewOverview
	case InputsFormDiscarded:
		m.viewState = ViewOverview
	}

	return m, cmd
}
