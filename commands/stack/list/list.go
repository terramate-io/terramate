// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package list provides the list command.
package list

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/terramate-io/terramate/cloud/api/status"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"
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
	Tags          []string
	NoTags        []string
	Printers      printer.Printers
	Format        string
}

// StatusFilters contains the status filters for the list command.
type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "list" }

// Exec executes the list command.
func (s *Spec) Exec(_ context.Context) error {
	if s.Reason && !s.GitFilter.IsChanged {
		return errors.E("the --why flag must be used together with --changed")
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
	tags, err := engine.ParseFilterTags(s.Tags, s.NoTags)
	if err != nil {
		return err
	}
	filteredStacks := s.Engine.FilterStacks(allStacks, tags)

	reasons := map[string]string{}
	stacks := make(config.List[*config.SortableStack], len(filteredStacks))
	for i, entry := range filteredStacks {
		stacks[i] = entry.Stack.Sortable()
		reasons[entry.Stack.ID] = entry.Reason
	}

	if s.RunOrder {
		var failReason string
		var err error
		failReason, err = run.Sort(s.Engine.Config(), stacks,
			func(s *config.SortableStack) *config.Stack { return s.Stack })
		if err != nil {
			return errors.E(err, "Invalid stack configuration: "+failReason)
		}

		if s.Format == "json" {
			graph, reason, err := run.BuildDAGFromStacks(s.Engine.Config(), stacks,
				func(s *config.SortableStack) *config.Stack { return s.Stack })
			if err != nil {
				return errors.E("Failed to build dependency graph: "+reason)
			}

			// Calculate levels based on DAG
			levelGroups := make(map[int][]string)
			stackLevels := make(map[string]int)

			// Process nodes in topological order
			order := graph.Order()
			for _, id := range order {
				stack, err := graph.Node(id)
				if err != nil {
					return errors.E("Failed to access node in graph: "+err.Error())
				}

				level := 1 // Default level
				// Check ancestors (dependencies)
				for _, ancestorID := range graph.AncestorsOf(id) {
					ancestorLevel := stackLevels[string(ancestorID)]
					if ancestorLevel >= level {
						level = ancestorLevel + 1
					}
				}

				dir := stack.Stack.Dir.String()
				stackLevels[dir] = level

				friendlyDir, ok := s.Engine.FriendlyFmtDir(dir)
				if !ok {
					printer.Stderr.Error(fmt.Sprintf("Unable to format stack dir %s", dir))
					continue
				}
				levelGroups[level] = append(levelGroups[level], friendlyDir)
			}

			jsonOutput, err := json.MarshalIndent(levelGroups, "", "  ")
			if err != nil {
				return errors.E("Error formatting stack groups as JSON: "+err.Error())
			}
			printer.Stdout.Println(string(jsonOutput))
			return nil
		}
	}

	// Original output format for non-runOrder case or when format is not json
	for _, st := range stacks {
		dir := st.Dir().String()
		friendlyDir, ok := s.Engine.FriendlyFmtDir(dir)
		if !ok {
			printer.Stderr.Error(fmt.Sprintf("Unable to format stack dir %s", dir))
			printer.Stdout.Println(dir)
			continue
		}
		if s.Reason {
			printer.Stdout.Println(fmt.Sprintf("%s - %s", friendlyDir, reasons[st.ID]))
		} else {
			printer.Stdout.Println(friendlyDir)
		}
	}
	return nil
}
