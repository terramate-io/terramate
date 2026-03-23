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
	"github.com/terramate-io/terramate/typeschema"
)

func (m Model) updateReconfigSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
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

// loadReconfigBundle loads the bundle definition for the given bundle,
// evaluates its input definitions, and creates the inputs form pre-populated
// with the bundle's current values.
func (m *Model) loadReconfigBundle(b *config.Bundle) error {
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

	m.reconfigBundle = b
	m.selectedBundleDefEntry = bde
	m.inputsForm = NewInputsFormWithValues(inputDefs, schemactx, est.Registry, m.selectedEnv, nil, values, values)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	return nil
}

// findBundleByLocation looks up a bundle in the registry by its location.
// When an environment is selected and the bundle has an environment, only
// the bundle matching the selected environment is returned.
func (m Model) findBundleByLocation(location string) *config.Bundle {
	for _, b := range m.EngineState.Registry.Bundles {
		if fmt.Sprintf("%s:%s", b.Workdir.String(), b.Name) != location {
			continue
		}
		if m.selectedEnv != nil && b.Environment != nil && b.Environment.ID != m.selectedEnv.ID {
			continue
		}
		return b
	}
	return nil
}

// loadBundleEvalContext creates a bundle eval context and loads the schema namespaces for the given bundle.
func (m Model) loadBundleEvalContext(bde *config.BundleDefinitionEntry) (typeschema.EvalContext, error) {
	est := m.EngineState
	evalctx := newBundleEvalContext(est.Evalctx, est.Registry.Registry, m.selectedEnv)
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
// ChangeModify entry in m.pendingChanges, sorted into grouped display order so that
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

		if m.selectedEnv != nil {
			// Show bundles from current environment, and those without an environment.
			if b.Environment != nil {
				if b.Environment.ID != m.selectedEnv.ID {
					continue
				}
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
// preserving the order of first appearance.
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
	return groups
}

// renderGroupedBundleItems renders each bundle group as a single renderedItem block
// and returns the group index containing the cursor, suitable for scrollWindowVar.
// contentWidth controls the fixed width each line is padded/truncated to so that
// the scrollbar column stays right-aligned.
func (m Model) renderGroupedBundleItems(groups []bundleGroup, cursor, contentWidth int) (int, []renderedItem) {
	est := m.EngineState

	idStyle := lipgloss.NewStyle().Foreground(colorTextSubtle)
	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	headerNameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	fromStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	lineStyle := lipgloss.NewStyle().Width(contentWidth)

	const colGap = 2

	type entryRow struct {
		left      string
		leftWidth int
		source    string
	}
	type groupData struct {
		headerLeft      string
		headerLeftWidth int
		headerSource    string
		entries         []entryRow
	}

	// First pass: build all rows and find the global max left width.
	allGroups := make([]groupData, len(groups))
	globalMaxLeft := 0
	selectedGroupIdx := 0
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
			isSelected := visualIdx == cursor
			if isSelected {
				selectedGroupIdx = gi
			}
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

		allGroups[gi] = groupData{
			headerLeft:      headerLeft,
			headerLeftWidth: headerLeftWidth,
			headerSource:    b0.Source,
			entries:         entries,
		}
	}

	// Second pass: render each group as a single block with fixed-width lines.
	items := make([]renderedItem, len(allGroups))
	for gi, gr := range allGroups {
		var lines []string
		headerPad := strings.Repeat(" ", globalMaxLeft-gr.headerLeftWidth+colGap)
		lines = append(lines, lineStyle.Render(gr.headerLeft+headerPad+fromStyle.Render(gr.headerSource)))
		for _, r := range gr.entries {
			entryPad := strings.Repeat(" ", globalMaxLeft-r.leftWidth+colGap)
			lines = append(lines, lineStyle.Render(r.left+entryPad+fromStyle.Render(r.source)))
		}
		block := strings.Join(lines, "\n")
		items[gi] = renderedItem{content: block, height: lipgloss.Height(block)}
	}

	return selectedGroupIdx, items
}

func (m Model) renderReconfigSelectView() string {
	panelWidth := m.effectiveWidth()
	innerWidth := panelWidth - 4
	scrollbarGutter := 4 // left gap(1) + scrollbar(1) + right gap(2)
	contentWidth := innerWidth - scrollbarGutter

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(panelWidth).
		Height(uiContentHeight + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	contentStyle := lipgloss.NewStyle().Width(innerWidth)

	title := m.renderHeader("reconfigure", panelWidth)

	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(colorText).MarginBottom(1).Render("Select a Bundle to Reconfigure")
	desc := lipgloss.NewStyle().Foreground(colorTextMuted).MarginBottom(2).Render("These bundles are currently deployed in your project.")

	header := lipgloss.JoinVertical(lipgloss.Left, sectionTitle, desc, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := uiContentHeight - headerHeight

	groups := groupBundles(m.reconfigBundles)
	selectedGroupIdx, items := m.renderGroupedBundleItems(groups, m.reconfigCursor, contentWidth)

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

func (m Model) renderReconfigInputView() string {
	panelWidth := m.effectiveWidth()
	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	b := m.reconfigBundle
	headerContext := fmt.Sprintf("reconfigure / %s", b.Name)
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
