// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"context"
	"fmt"
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

func (m Model) updateCreateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	est := m.EngineState
	switch {
	case key.Matches(msg, keys.Escape):
		if m.bundleSelectPage == BundleSelectCollection {
			m.viewState = ViewOverview
		} else {
			m.bundleSelectPage--
		}
		return m, nil

	case key.Matches(msg, keys.Up):
		switch m.bundleSelectPage {
		case BundleSelectCollection:
			if m.selectedCollIdx > 0 {
				m.selectedCollIdx--
			}
		case BundleSelectBundle:
			if m.selectedBundleIdx > 0 {
				m.selectedBundleIdx--
			}
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		switch m.bundleSelectPage {
		case BundleSelectCollection:
			if m.selectedCollIdx < len(est.Collections)-1 {
				m.selectedCollIdx++
			}
		case BundleSelectBundle:
			bundles := est.Collections[m.selectedCollIdx].Bundles
			if m.selectedBundleIdx < len(bundles)-1 {
				m.selectedBundleIdx++
			}
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		switch m.bundleSelectPage {
		case BundleSelectCollection:
			m.selectedBundleIdx = 0
			m.bundleSelectPage = BundleSelectBundle
		case BundleSelectBundle:
			return m.selectBundle()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) selectBundle() (tea.Model, tea.Cmd) {
	if err := m.loadBundleDef(m.selectedCollIdx, m.selectedBundleIdx); err != nil {
		return m.updateError(err)
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
		bde = &est.LocalBundleDefs[m.selectedBundleIdx]
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

func (m Model) renderBundleSelectView() string {
	est := m.EngineState
	innerWidth := uiWidth - 4

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(uiWidth).
		Height(uiContentHeight + 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	contentStyle := lipgloss.NewStyle().
		Width(innerWidth)

	collName := strings.ToLower(est.Collections[m.selectedCollIdx].Name)

	var headerContext string
	switch m.bundleSelectPage {
	case BundleSelectCollection:
		headerContext = "add bundle"
	case BundleSelectBundle:
		headerContext = fmt.Sprintf("add bundle / %s", collName)
	}

	title := m.renderHeader(headerContext)

	helpText := m.finalHelpText("esc: back")
	help := helpStyle.Render(helpText)

	var content string
	switch m.bundleSelectPage {
	case BundleSelectCollection:
		content = m.renderCollectionPage()
	case BundleSelectBundle:
		content = m.renderBundlePage()
	}

	section := borderStyle.Render(contentStyle.Render(content))

	all := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		section,
		help,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(all)
}

type renderedItem struct {
	content string
	height  int
}

// scrollWindowVar computes a visible window of variable-height items that fits
// within availableHeight, keeping the selected item visible.
// Each item beyond the first costs an extra line for the inter-item separator.
func scrollWindowVar(selectedIdx int, items []renderedItem, availableHeight int) (start, end int) {
	total := len(items)
	if total == 0 {
		return 0, 0
	}

	sep := 1 // blank line between items ("\n\n" adds 1 visual line)
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

func (m Model) renderCollectionPage() string {
	est := m.EngineState
	innerWidth := uiWidth - 4
	scrollbarGutter := 4 // left gap(1) + scrollbar(1) + right gap(2)
	contentWidth := innerWidth - scrollbarGutter

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		MarginBottom(1)

	descStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		MarginBottom(1)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(contentWidth)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(colorPrimary).
		Bold(true).
		Width(contentWidth)

	itemDescStyle := lipgloss.NewStyle().
		PaddingLeft(4).
		Foreground(colorTextMuted).
		Width(contentWidth)

	lastUsedStyle := lipgloss.NewStyle().
		Foreground(colorTextSubtle).
		Italic(true)

	title := titleStyle.Render("Select Collection")
	desc := descStyle.Render("Choose a collection containing bundles")
	header := lipgloss.JoinVertical(lipgloss.Left, title, desc, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := uiContentHeight - headerHeight

	var items []renderedItem
	for i, coll := range est.Collections {
		name := coll.Name
		if m.hasLastUsedColl && i == m.lastUsedCollIdx {
			name = name + " " + lastUsedStyle.Render("(last used)")
		}
		var line string
		if i == m.selectedCollIdx {
			line = selectedStyle.Render("› " + name)
		} else {
			line = itemStyle.Render("  " + name)
		}
		block := line + "\n" + itemDescStyle.Render(summaryLine(coll.Description))
		items = append(items, renderedItem{content: block, height: lipgloss.Height(block)})
	}

	start, end := scrollWindowVar(m.selectedCollIdx, items, availableHeight)

	var sb strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			sb.WriteString("\n\n")
		}
		sb.WriteString(items[i].content)
	}
	listContent := sb.String()

	if len(est.Collections) > end-start {
		trackHeight := lipgloss.Height(listContent)
		scrollbar := renderScrollbar(len(est.Collections), end-start, start, trackHeight)
		listContent = lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", scrollbar, "  ")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, listContent)
}

func (m Model) renderBundlePage() string {
	est := m.EngineState
	innerWidth := uiWidth - 4
	scrollbarGutter := 4 // left gap(1) + scrollbar(1) + right gap(2)
	contentWidth := innerWidth - scrollbarGutter

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		MarginBottom(1)

	descStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		MarginBottom(1)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(contentWidth)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(colorPrimary).
		Bold(true).
		Width(contentWidth)

	itemDescStyle := lipgloss.NewStyle().
		PaddingLeft(4).
		Foreground(colorTextMuted).
		Width(contentWidth)

	versionStyle := lipgloss.NewStyle().
		Foreground(colorTextSubtle)

	coll := est.Collections[m.selectedCollIdx]
	title := titleStyle.Render("Select Bundle")
	desc := descStyle.Render(fmt.Sprintf("Bundles in %s", coll.Name))
	header := lipgloss.JoinVertical(lipgloss.Left, title, desc, "")
	headerHeight := lipgloss.Height(header)
	availableHeight := uiContentHeight - headerHeight

	var items []renderedItem
	for i, bundle := range coll.Bundles {
		displayName := fmt.Sprintf("%s %s", bundle.Name, versionStyle.Render("v"+bundle.Version))
		var line string
		if i == m.selectedBundleIdx {
			line = selectedStyle.Render("› " + displayName)
		} else {
			line = itemStyle.Render("  " + displayName)
		}
		block := line
		if bundle.Description != "" {
			block += "\n" + itemDescStyle.Render(summaryLine(strings.TrimSpace(bundle.Description)))
		}
		items = append(items, renderedItem{content: block, height: lipgloss.Height(block)})
	}

	start, end := scrollWindowVar(m.selectedBundleIdx, items, availableHeight)

	var sb strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			sb.WriteString("\n\n")
		}
		sb.WriteString(items[i].content)
	}
	listContent := sb.String()

	if len(coll.Bundles) > end-start {
		trackHeight := lipgloss.Height(listContent)
		scrollbar := renderScrollbar(len(coll.Bundles), end-start, start, trackHeight)
		listContent = lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", scrollbar, "  ")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, listContent)
}
