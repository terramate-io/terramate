// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/scaffold/manifest"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/typeschema"
	"github.com/zclconf/go-cty/cty"
)

// buildFlatBundles creates a flat list of all bundles from all collections,
// sorted by name then version.
func buildFlatBundles(est *EngineState) []flatBundleEntry {
	hasLocal := len(est.LocalBundleDefs) > 0
	var entries []flatBundleEntry
	for collIdx, coll := range est.Collections {
		for bundleIdx := range coll.Bundles {
			entries = append(entries, flatBundleEntry{
				collIdx:   collIdx,
				bundleIdx: bundleIdx,
				bundle:    &coll.Bundles[bundleIdx],
				collName:  coll.Name,
				isLocal:   hasLocal && collIdx == 0,
			})
		}
	}
	slices.SortFunc(entries, func(a, b flatBundleEntry) int {
		if c := cmp.Compare(a.bundle.Name, b.bundle.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(b.bundle.Version, a.bundle.Version); c != 0 {
			return c // descending version (latest first)
		}
		return cmp.Compare(a.collName, b.collName)
	})
	return entries
}

func (m Model) updateCreateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.viewState = ViewOverview
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.flatBundleCursor > 0 {
			m.flatBundleCursor--
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.flatBundleCursor < len(m.flatBundles)-1 {
			m.flatBundleCursor++
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		return m.selectFlatBundle()
	}

	return m, nil
}

func (m Model) selectFlatBundle() (tea.Model, tea.Cmd) {
	entry := m.flatBundles[m.flatBundleCursor]
	m.selectedCollIdx = entry.collIdx
	m.selectedBundleIdx = entry.bundleIdx
	m.selectedEnv = nil // reset so env picker shows for each bundle

	if err := m.loadBundleDef(entry.collIdx, entry.bundleIdx); err != nil {
		return m.updateErrorWithTitle("Bundle is not enabled", err)
	}

	// If the bundle requires an environment and environments are configured,
	// show the env picker before proceeding to inputs.
	if bundleRequiresEnv(m.EngineState.Evalctx, m.selectedBundleDefEntry.Define) &&
		len(m.EngineState.Registry.Environments) > 0 {
		m.createEnvCursor = 0
		m.viewState = ViewCreateEnvSelect
		return m, nil
	}

	m.viewState = ViewCreateInput
	return m, m.inputsForm.FocusActiveInput()
}

// loadBundleDef loads the bundle definition at the given collection/bundle index,
// evaluates its inputs/schemas, and prepares the inputs form. Used by both the
// initial bundle selection flow and nested bundle-ref creation.
func (m *Model) loadBundleDef(collIdx, bundleIdx int) error {
	est := m.EngineState
	var bde *config.BundleDefinitionEntry
	var source string

	if len(est.LocalBundleDefs) > 0 && collIdx == 0 {
		// Local "fake" collection. No need to download anything, we have the files already.
		bde = &est.LocalBundleDefs[bundleIdx]
		source = bde.Tree.Dir().String()
	} else {
		// Remote collection. We only have the manifest metadata and get the whole bundle now.
		coll := est.Collections[collIdx]
		collBundle := &coll.Bundles[bundleIdx]

		source = bundleSourceFromManifest(coll, collBundle)

		bundleDir, err := est.ResolveAPI.Resolve(est.Root.HostDir(), source, resolve.Bundle, true)
		if err != nil {
			return errors.E("Failed to resolve bundle")
		}

		tree, define, err := config.LoadSingleBundleDefinition(est.Root, bundleDir)
		if err != nil {
			return errors.E("Failed to load bundle")
		}

		md, err := config.EvalMetadata(est.Evalctx, tree, &define.Metadata)
		if err != nil {
			return errors.E("Failed to load bundle")
		}

		bde = &config.BundleDefinitionEntry{
			Source:   source,
			Tree:     tree,
			Define:   define,
			Metadata: md,
		}
	}

	if err := checkEnvRequired(est.Evalctx, bde.Define, est.Registry.Environments); err != nil {
		return err
	}

	// If the bundle requires an environment but none is selected yet,
	// defer checkBundleEnabled and schema evaluation to finalizeBundleWithEnv
	// which runs after the user picks an environment.
	if bundleRequiresEnv(est.Evalctx, bde.Define) && m.selectedEnv == nil && len(est.Registry.Environments) > 0 {
		m.selectedCollIdx = collIdx
		m.selectedBundleIdx = bundleIdx
		m.selectedBundleDefEntry = bde
		m.selectedBundleSource = source
		return nil
	}

	bundleEvalctx := newBundleEvalContext(est.Evalctx, est.Registry.Registry, m.selectedEnv)

	if err := checkBundleEnabled(bundleEvalctx, bde.Define); err != nil {
		return err
	}

	schemas, err := config.EvalBundleSchemaNamespaces(est.Root, est.ResolveAPI, bundleEvalctx, bde.Define, true)
	if err != nil {
		return errors.E(err, "Failed to load bundle schema")
	}

	schemactx := typeschema.EvalContext{
		Evalctx: bundleEvalctx,
		Schemas: schemas,
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errors.E(err, "Failed to evaluate input definitions")
	}

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

	m.selectedCollIdx = collIdx
	m.selectedBundleIdx = bundleIdx
	m.selectedBundleDefEntry = bde
	m.selectedBundleSource = source
	m.inputsForm = NewInputsForm(inputDefs, schemactx, est.Registry, m.selectedEnv)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
	return nil
}

// finalizeBundleWithEnv re-evaluates the already-loaded bundle definition with
// the newly selected environment. Called after the deferred env picker.
func (m *Model) finalizeBundleWithEnv() error {
	est := m.EngineState
	bde := m.selectedBundleDefEntry

	bundleEvalctx := newBundleEvalContext(est.Evalctx, est.Registry.Registry, m.selectedEnv)

	if err := checkBundleEnabled(bundleEvalctx, bde.Define); err != nil {
		return err
	}

	schemas, err := config.EvalBundleSchemaNamespaces(est.Root, est.ResolveAPI, bundleEvalctx, bde.Define, true)
	if err != nil {
		return errors.E(err, "Failed to load bundle schema")
	}

	schemactx := typeschema.EvalContext{
		Evalctx: bundleEvalctx,
		Schemas: schemas,
	}

	inputDefs, err := config.EvalBundleInputDefinitions(schemactx, bde.Define)
	if err != nil {
		return errors.E(err, "Failed to evaluate input definitions")
	}

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

	m.inputsForm = NewInputsForm(inputDefs, schemactx, est.Registry, m.selectedEnv)
	m.inputsForm.PanelWidth = m.effectiveWidth()
	m.inputsForm.PanelHeight = m.effectiveInputsPanelHeight()
	return nil
}

func bundleSourceFromManifest(coll *manifest.Collection, bundle *manifest.Bundle) string {
	addr, params, found := strings.Cut(coll.Location, "?")
	if found {
		return fmt.Sprintf("%s//%s?%s", addr, bundle.Path, params)
	}
	return fmt.Sprintf("%s//%s", addr, bundle.Path)
}

func bundleRequiresEnv(evalctx *eval.Context, def *hcl.DefineBundle) bool {
	if def.Environments.Required == nil {
		return false
	}
	envRequired, err := config.EvalBool(evalctx, def.Environments.Required.Expr, "environments.required")
	if err != nil {
		return false
	}
	return envRequired
}

func checkEnvRequired(evalctx *eval.Context, def *hcl.DefineBundle, envs []*config.Environment) error {
	if bundleRequiresEnv(evalctx, def) && len(envs) == 0 {
		return errors.E("This bundle requires environments, but none are configured.")
	}
	return nil
}

func newBundleEvalContext(evalctx *eval.Context, reg *config.Registry, env *config.Environment) *eval.Context {
	evalctx = evalctx.ChildContext()

	var bundleVals map[string]cty.Value
	if bundleNS, ok := evalctx.GetNamespace("bundle"); ok {
		bundleVals = bundleNS.AsValueMap()
	} else {
		bundleVals = map[string]cty.Value{}
	}
	bundleVals["environment"] = config.MakeEnvObject(env)
	evalctx.SetNamespace("bundle", bundleVals)

	evalctx.SetFunction(stdlib.Name("bundle"), config.BundleFunc(context.TODO(), reg, env, false))
	evalctx.SetFunction(stdlib.Name("bundles"), config.BundlesFunc(reg, env))
	return evalctx
}

func checkBundleEnabled(evalctx *eval.Context, def *hcl.DefineBundle) error {
	for _, cond := range def.Scaffolding.Enabled {
		enabled, err := config.EvalBool(evalctx, cond.Condition.Expr, "scaffolding.enabled.condition")
		if err != nil {
			return errors.E(err, cond.Condition.Range)
		}
		errorMsg, err := config.EvalString(evalctx, cond.ErrorMessage.Expr, "scaffolding.enabled.error_message")
		if err != nil {
			return errors.E(err, cond.ErrorMessage.Range)
		}
		if !enabled {
			return errors.E(errorMsg)
		}
	}
	return nil
}

// --- Deferred environment selection for Create flow ---

func (m Model) updateCreateEnvSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState
	switch {
	case key.Matches(msg, keys.Escape):
		m.selectedEnv = nil
		m.viewState = ViewCreateSelect
		return m, nil

	case key.Matches(msg, keys.Up):
		if m.createEnvCursor > 0 {
			m.createEnvCursor--
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		if m.createEnvCursor < len(est.Registry.Environments)-1 {
			m.createEnvCursor++
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		m.selectedEnv = est.Registry.Environments[m.createEnvCursor]
		if err := m.finalizeBundleWithEnv(); err != nil {
			m.selectedEnv = nil // rollback so env picker shows again
			return m.updateErrorWithTitle("Bundle is not enabled", err)
		}
		m.viewState = ViewCreateInput
		return m, m.inputsForm.FocusActiveInput()
	}
	return m, nil
}

func (m Model) renderCreateEnvSelectView() string {
	est := m.EngineState
	panelWidth := m.effectiveWidth()

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(panelWidth).
		Height(m.effectiveContentHeight() + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(panelWidth)

	innerWidth := panelWidth - 4
	contentWidth := innerWidth - 4 // scrollbar gutter

	bundleName := ""
	if m.flatBundleCursor < len(m.flatBundles) {
		bundleName = m.flatBundles[m.flatBundleCursor].bundle.Name
	}
	header := m.renderHeader(fmt.Sprintf("Create %s", bundleName), panelWidth)

	// Bundle detail box — full details plus currently highlighted environment
	var detailBox string
	if m.flatBundleCursor < len(m.flatBundles) {
		entry := m.flatBundles[m.flatBundleCursor]
		fields := []detailField{
			{label: "Bundle", value: entry.bundle.Name + " v" + entry.bundle.Version, truncEnd: true},
		}
		if entry.bundle.Class != "" {
			fields = append(fields, detailField{label: "Class", value: entry.bundle.Class, truncEnd: true})
		}
		// Show currently highlighted environment
		if m.createEnvCursor < len(est.Registry.Environments) {
			fields = append(fields, detailField{label: "Environment", value: est.Registry.Environments[m.createEnvCursor].Name, truncEnd: true})
		}
		fields = append(fields, detailField{}) // separator
		coll := est.Collections[entry.collIdx]
		collDetail := coll.Name
		if coll.Location != "" {
			collDetail += " (" + coll.Location + ")"
		}
		fields = append(fields, detailField{label: "Collection", value: collDetail})
		var source string
		if entry.isLocal {
			source = est.LocalBundleDefs[entry.bundleIdx].Tree.Dir().String()
		} else {
			source = bundleSourceFromManifest(coll, entry.bundle)
		}
		fields = append(fields, detailField{label: "Source", value: source})
		detailBox = renderDetailBox(contentWidth, "Bundle Details", fields)
	}

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

	var items []string
	for i, env := range est.Registry.Environments {
		idTag := idStyle.Render("[" + env.ID + "]")
		if i == m.createEnvCursor {
			items = append(items, selectedStyle.Render("› "+env.Name)+" "+idTag)
		} else {
			items = append(items, itemStyle.Render("  "+env.Name)+" "+idTag)
		}
		items = append(items, itemDescStyle.Render(env.Description))
		items = append(items, "")
	}

	panelContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{detailBox, ""}, items...)...)
	panel := borderStyle.Render(panelContent)

	help := helpStyle.Render(m.finalHelpText("↑↓: Select Environment • esc: back"))

	content := lipgloss.JoinVertical(lipgloss.Left, header, panel, help)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

// --- Flat bundle list rendering ---

func (m Model) renderBundleSelectView() string {
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

	contentStyle := lipgloss.NewStyle().
		Width(innerWidth)

	title := m.renderHeader("Create Bundle Instance", panelWidth)

	help := helpStyle.Render(m.finalHelpText("↑↓: Select Bundle • esc: back"))

	content := m.renderFlatBundleList(contentWidth)

	section := borderStyle.Render(contentStyle.Render(content))

	all := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		section,
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(all)
}

// detailField represents a labeled field in the detail box.
type detailField struct {
	label    string
	value    string
	truncEnd bool // true: truncate end ("long..."), false: truncate start ("...long")
}

// renderDetailBox renders a sticky detail box with a titled border and labeled fields.
// The boxTitle appears on the top border line. The first field is the main title (bold).
func renderDetailBox(innerWidth int, boxTitle string, fields []detailField) string {
	borderColor := lipgloss.NewStyle().Foreground(colorBorder)
	titleColor := lipgloss.NewStyle().Foreground(colorTextMuted)
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	labelStyle := lipgloss.NewStyle().Foreground(colorTextSubtle)
	valueStyle := lipgloss.NewStyle().Foreground(colorTextMuted)
	versionStyle := lipgloss.NewStyle().Foreground(colorTextSubtle)

	// Content width inside the box (border 1 + padding 1 on each side)
	contentWidth := innerWidth - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Build the top border with title: ╭─ Title ───────────╮
	titleText := titleColor.Render(" " + boxTitle + " ")
	titleVisualWidth := lipgloss.Width(titleText)
	fillWidth := innerWidth - 2 - 1 - titleVisualWidth // 2 for corners, 1 for dash before title
	if fillWidth < 0 {
		fillWidth = 0
	}
	topBorder := borderColor.Render("╭─") + titleText + borderColor.Render(strings.Repeat("─", fillWidth)+"╮")

	// Find max label width for alignment
	maxLabelWidth := 0
	for _, f := range fields {
		if f.label != "" && lipgloss.Width(f.label)+2 > maxLabelWidth {
			maxLabelWidth = lipgloss.Width(f.label) + 2 // +2 for ": "
		}
	}

	// Build content lines
	var contentLines []string
	for _, f := range fields {
		// Separator line
		if f.label == "" && f.value == "" {
			sepLine := borderColor.Render("├" + strings.Repeat("─", innerWidth-2) + "┤")
			contentLines = append(contentLines, sepLine)
			continue
		}
		var line string
		if f.label != "" {
			paddedLabel := f.label + ": " + strings.Repeat(" ", maxLabelWidth-len(f.label)-2)
			label := labelStyle.Render(paddedLabel)
			labelWidth := lipgloss.Width(label)
			availWidth := contentWidth - labelWidth
			if availWidth < 10 {
				availWidth = 10
			}
			// Split name+version for the Bundle field
			name, version := splitNameVersion(f.value)
			if version != "" {
				val := truncateEnd(name, availWidth-len(version)-1)
				line = label + nameStyle.Render(val) + " " + versionStyle.Render(version)
			} else {
				var val string
				if f.truncEnd {
					val = truncateEnd(f.value, availWidth)
				} else {
					val = truncateStart(f.value, availWidth)
				}
				line = label + valueStyle.Render(val)
			}
		} else {
			name, version := splitNameVersion(f.value)
			line = nameStyle.Render(truncateEnd(name, contentWidth-len(version)-1))
			if version != "" {
				line += " " + versionStyle.Render(version)
			}
		}
		// Pad each line to full width and wrap with border chars
		lineWidth := lipgloss.Width(line)
		pad := contentWidth - lineWidth
		if pad < 0 {
			pad = 0
		}
		contentLines = append(contentLines, borderColor.Render("│")+" "+line+strings.Repeat(" ", pad)+" "+borderColor.Render("│"))
	}

	// Bottom border
	bottomBorder := borderColor.Render("╰" + strings.Repeat("─", innerWidth-2) + "╯")

	all := []string{topBorder}
	all = append(all, contentLines...)
	all = append(all, bottomBorder)
	return strings.Join(all, "\n")
}

// splitNameVersion splits "Some Name vX.Y.Z" into ("Some Name", "vX.Y.Z").
func splitNameVersion(s string) (string, string) {
	// Find the last " v" followed by a digit
	for i := len(s) - 1; i > 0; i-- {
		if s[i-1] == ' ' && s[i] == 'v' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			return s[:i-1], s[i:]
		}
	}
	return s, ""
}

// truncateEnd truncates a string at the end: "very long string" → "very long st..."
func truncateEnd(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return s
	}
	if len(s) <= maxWidth {
		return s
	}
	return s[:maxWidth-3] + "..."
}

// truncateStart truncates a string at the start: "/very/long/path/file" → ".../long/path/file"
func truncateStart(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return s
	}
	if len(s) <= maxWidth {
		return s
	}
	return "..." + s[len(s)-maxWidth+3:]
}

func (m Model) renderFlatBundleList(contentWidth int) string {
	// Detail box for the highlighted bundle
	var detailBox string
	if m.flatBundleCursor < len(m.flatBundles) {
		est := m.EngineState
		entry := m.flatBundles[m.flatBundleCursor]
		fields := []detailField{
			{label: "Bundle", value: entry.bundle.Name + " v" + entry.bundle.Version, truncEnd: true},
		}
		if entry.bundle.Class != "" {
			fields = append(fields, detailField{label: "Class", value: entry.bundle.Class, truncEnd: true})
		}
		fields = append(fields, detailField{}) // separator
		// Collection: name and location
		coll := est.Collections[entry.collIdx]
		collDetail := coll.Name
		if coll.Location != "" {
			collDetail += " (" + coll.Location + ")"
		}
		fields = append(fields, detailField{label: "Collection", value: collDetail})
		// Source: the real resolved source path to the bundle definition
		var source string
		if entry.isLocal {
			source = est.LocalBundleDefs[entry.bundleIdx].Tree.Dir().String()
		} else {
			source = bundleSourceFromManifest(coll, entry.bundle)
		}
		fields = append(fields, detailField{label: "Source", value: source})
		detailBox = renderDetailBox(contentWidth, "Bundle Details", fields)
	}

	header := lipgloss.JoinVertical(lipgloss.Left, detailBox, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := m.effectiveContentHeight() - headerHeight

	itemStyle := lipgloss.NewStyle().
		Bold(true).
		Width(contentWidth)

	selectedStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Width(contentWidth)

	versionStyle := lipgloss.NewStyle().
		Foreground(colorTextSubtle)

	descStyle := lipgloss.NewStyle().
		PaddingLeft(6).
		Foreground(colorTextMuted).
		Width(contentWidth)

	var items []renderedItem
	for i, entry := range m.flatBundles {
		displayName := fmt.Sprintf("%s %s", entry.bundle.Name, versionStyle.Render("v"+entry.bundle.Version))

		var line string
		if i == m.flatBundleCursor {
			line = selectedStyle.Render("› " + displayName)
		} else {
			line = itemStyle.Render("  " + displayName)
		}

		block := line
		if entry.bundle.Description != "" {
			block += "\n" + descStyle.Render(summaryLine(strings.TrimSpace(entry.bundle.Description)))
		}
		items = append(items, renderedItem{content: block, height: lipgloss.Height(block)})
	}

	start, end := scrollWindowVar(m.flatBundleCursor, items, availableHeight, 1)

	var sb strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			sb.WriteString("\n\n")
		}
		sb.WriteString(items[i].content)
	}
	listContent := sb.String()

	if len(m.flatBundles) > end-start {
		trackHeight := lipgloss.Height(listContent)
		scrollbar := renderScrollbar(len(m.flatBundles), end-start, start, trackHeight)
		listContent = lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", scrollbar, "  ")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, listContent)
}

type renderedItem struct {
	content string
	height  int
}

// scrollWindowVar computes a visible window of variable-height items that fits
// within availableHeight, keeping the selected item visible.
// sep is the number of visual lines between items (1 for "\n\n", 0 for "\n").
func scrollWindowVar(selectedIdx int, items []renderedItem, availableHeight, sep int) (start, end int) {
	total := len(items)
	if total == 0 {
		return 0, 0
	}
	totalH := 0
	for i, it := range items {
		totalH += it.height
		if i > 0 {
			totalH += sep
		}
	}
	if totalH <= availableHeight {
		return 0, total
	}

	// Start from selected, expand downward then upward.
	start = selectedIdx
	end = selectedIdx + 1
	usedH := items[selectedIdx].height

	for {
		expanded := false
		if end < total && usedH+sep+items[end].height <= availableHeight {
			usedH += sep + items[end].height
			end++
			expanded = true
		}
		if start > 0 && usedH+sep+items[start-1].height <= availableHeight {
			start--
			usedH += sep + items[start].height
			expanded = true
		}
		if !expanded {
			break
		}
	}
	return
}

func renderScrollbar(totalItems, visibleCount, offset, trackHeight int) string {
	if totalItems <= visibleCount || trackHeight <= 0 {
		return ""
	}

	thumbSize := max(1, trackHeight*visibleCount/totalItems)
	maxOff := totalItems - visibleCount
	thumbPos := 0
	if maxOff > 0 {
		thumbPos = (trackHeight - thumbSize) * offset / maxOff
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
