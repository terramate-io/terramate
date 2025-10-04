// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package list provides the list command.
package list

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emicklei/dot"
	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"
	"github.com/terramate-io/terramate/run/dag"
	"github.com/terramate-io/terramate/stack"
)

// Spec is the command specification for the list command.
type Spec struct {
	Engine        *engine.Engine
	GitFilter     engine.GitFilter
	Reason        bool
	Target        string
	StatusFilters StatusFilters
	RunOrder      bool
	Format        string
	Label         string
	Tags          []string
	NoTags        []string
	Printers      printer.Printers
}

// StatusFilters contains the status filters for the list command.
type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

// StackInfo represents stack information for JSON output.
type StackInfo struct {
	Dir          string   `json:"dir"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	Dependencies []string `json:"dependencies"`
	Reason       string   `json:"reason"`
	IsChanged    bool     `json:"is_changed"`
}

// stackMetadata holds processed stack information for output formatting.
type stackMetadata struct {
	Stack       *config.SortableStack
	FriendlyDir string
	Label       string
	Entry       stack.Entry
	AncestorIDs []dag.ID
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "list" }

// getLabel extracts the appropriate label from a stack based on the Label field.
func (s *Spec) getLabel(st *config.Stack, friendlyDir string) (string, error) {
	switch s.Label {
	case "stack.name":
		return st.Name, nil
	case "stack.id":
		return st.ID, nil
	case "stack.dir":
		return friendlyDir, nil
	default:
		return "", errors.E(`--label expects the values "stack.name", "stack.id", or "stack.dir"`)
	}
}

// Exec executes the list command.
func (s *Spec) Exec(_ context.Context) error {
	if s.Reason && !s.GitFilter.IsChanged {
		return errors.E("the --why flag must be used together with --changed")
	}

	if s.Format == "json" && s.Label == "stack.name" {
		return errors.E("--format json cannot be used with --label stack.name (stack names are not guaranteed to be unique)")
	}

	err := s.Engine.CheckTargetsConfiguration(s.Target, "", func(isTargetSet bool) error {
		isStatusSet := s.StatusFilters.StackStatus != ""
		isDeploymentStatusSet := s.StatusFilters.DeploymentStatus != ""
		isDriftStatusSet := s.StatusFilters.DriftStatus != ""

		if isTargetSet && (!isStatusSet && !isDeploymentStatusSet && !isDriftStatusSet) {
			return errors.E("--target must be used together with --status or --deployment-status or --drift-status")
		} else if !isTargetSet && (isStatusSet || isDeploymentStatusSet || isDriftStatusSet) {
			return errors.E("--status, --deployment-status and --drift-status requires --target when terramate.config.cloud.targets.enabled is true")
		}
		return nil
	})
	if err != nil {
		return err
	}

	cloudFilters, err := status.ParseFilters(s.StatusFilters.StackStatus, s.StatusFilters.DeploymentStatus, s.StatusFilters.DriftStatus)
	if err != nil {
		return err
	}

	report, err := s.Engine.ListStacks(s.GitFilter, s.Target, cloudFilters, false)
	if err != nil {
		return err
	}
	return s.printStacksList(report.Stacks)
}

func (s *Spec) printStacksList(allStacks []stack.Entry) error {
	filteredStacks, err := s.Engine.FilterStacks(allStacks,
		engine.ByWorkingDir(),
		engine.ByTags(s.Tags, s.NoTags),
	)
	if err != nil {
		return err
	}

	reasons := map[string]string{}
	stacks := make(config.List[*config.SortableStack], len(filteredStacks))
	for i, entry := range filteredStacks {
		stacks[i] = entry.Stack.Sortable()
		reasons[entry.Stack.ID] = entry.Reason
	}

	if s.Format == "json" {
		return s.printStacksListJSON(stacks, filteredStacks)
	}

	if s.Format == "dot" {
		return s.printStacksListDot(stacks, filteredStacks)
	}

	if s.RunOrder {
		var failReason string
		var err error
		failReason, err = run.Sort(s.Engine.Config(), stacks,
			func(s *config.SortableStack) *config.Stack { return s.Stack })
		if err != nil {
			return errors.E(err, "Invalid stack configuration: "+failReason)
		}
	}

	for _, st := range stacks {
		dir := st.Dir().String()
		friendlyDir, ok := s.Engine.FriendlyFmtDir(dir)
		if !ok {
			printer.Stderr.Error(fmt.Sprintf("Unable to format stack dir %s", dir))
			printer.Stdout.Println(dir)
			continue
		}

		label, err := s.getLabel(st.Stack, friendlyDir)
		if err != nil {
			return err
		}

		if s.Reason {
			printer.Stdout.Println(fmt.Sprintf("%s - %s", label, reasons[st.ID]))
		} else {
			printer.Stdout.Println(label)
		}
	}
	return nil
}

// buildStackMetadata processes stacks and collects metadata to be formatted.
func (s *Spec) buildStackMetadata(stacks config.List[*config.SortableStack], filteredStacks []stack.Entry) (map[dag.ID]*stackMetadata, *dag.DAG[*config.SortableStack], error) {
	// Create a map from stack ID to Entry for quick lookup
	entryMap := make(map[string]stack.Entry)
	for _, entry := range filteredStacks {
		entryMap[entry.Stack.ID] = entry
	}

	d, reason, err := run.BuildDAGFromStacks(
		s.Engine.Config(),
		stacks,
		func(s *config.SortableStack) *config.Stack { return s.Stack },
	)
	if err != nil {
		return nil, nil, errors.E(err, "Invalid stack configuration: "+reason)
	}

	metadata := make(map[dag.ID]*stackMetadata)

	for _, id := range d.IDs() {
		st, err := d.Node(id)
		if err != nil {
			return nil, nil, errors.E(err, "getting node from DAG")
		}

		dir := st.Stack.Dir.String()
		friendlyDir, ok := s.Engine.FriendlyFmtDir(dir)
		if !ok {
			return nil, nil, errors.E("unable to format stack dir %s", dir)
		}

		label, err := s.getLabel(st.Stack, friendlyDir)
		if err != nil {
			return nil, nil, err
		}

		ancestors := d.DirectAncestorsOf(id)

		entry, hasEntry := entryMap[st.Stack.ID]
		if !hasEntry {
			entry = stack.Entry{Stack: st.Stack}
		}

		metadata[id] = &stackMetadata{
			Stack:       st,
			FriendlyDir: friendlyDir,
			Label:       label,
			Entry:       entry,
			AncestorIDs: ancestors,
		}
	}

	return metadata, d, nil
}

func (s *Spec) printStacksListJSON(stacks config.List[*config.SortableStack], filteredStacks []stack.Entry) error {
	metadata, d, err := s.buildStackMetadata(stacks, filteredStacks)
	if err != nil {
		return err
	}

	stackInfos := make(map[string]StackInfo)

	for _, id := range d.IDs() {
		m := metadata[id]

		// Build dependency labels from ancestor IDs
		deps := make([]string, 0, len(m.AncestorIDs))
		for _, ancestorID := range m.AncestorIDs {
			ancestorMeta := metadata[ancestorID]
			deps = append(deps, ancestorMeta.Label)
		}

		info := StackInfo{
			Dir:          m.FriendlyDir,
			ID:           m.Stack.Stack.ID,
			Name:         m.Stack.Stack.Name,
			Description:  m.Stack.Stack.Description,
			Tags:         m.Stack.Stack.Tags,
			Dependencies: deps,
			Reason:       m.Entry.Reason,
			IsChanged:    m.Stack.Stack.IsChanged,
		}

		stackInfos[m.Label] = info
	}

	jsonData, err := json.MarshalIndent(stackInfos, "", "  ")
	if err != nil {
		return errors.E(err, "marshaling JSON")
	}

	s.Printers.Stdout.Println(string(jsonData))
	return nil
}

func (s *Spec) printStacksListDot(stacks config.List[*config.SortableStack], filteredStacks []stack.Entry) error {
	metadata, d, err := s.buildStackMetadata(stacks, filteredStacks)
	if err != nil {
		return err
	}

	dotGraph := dot.NewGraph(dot.Directed)

	for _, id := range d.IDs() {
		m := metadata[id]

		descendant := dotGraph.Node(m.FriendlyDir)
		if m.Label != m.FriendlyDir {
			descendant.Attr("label", m.Label)
		}

		for _, ancestorID := range m.AncestorIDs {
			ancestorMeta := metadata[ancestorID]

			ancestorNode := dotGraph.Node(ancestorMeta.FriendlyDir)
			if ancestorMeta.Label != ancestorMeta.FriendlyDir {
				ancestorNode.Attr("label", ancestorMeta.Label)
			}

			// check if edge already exists to avoid duplicates
			edges := dotGraph.FindEdges(ancestorNode, descendant)
			if len(edges) == 0 {
				dotGraph.Edge(ancestorNode, descendant)
			}
		}
	}

	s.Printers.Stdout.Println(dotGraph.String())
	return nil
}
