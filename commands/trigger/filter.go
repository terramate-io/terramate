// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package trigger

import (
	"context"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/printer"

	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/cloud/api/status"
)

// StatusFilters represents the status filters for stacks.
type StatusFilters struct {
	StackStatus      string
	DeploymentStatus string
	DriftStatus      string
}

// FilterSpec represents the trigger filter specification.
type FilterSpec struct {
	GitFilter     engine.GitFilter
	StatusFilters StatusFilters
	Tags          []string
	NoTags        []string
	Change        bool
	IgnoreChange  bool
	Reason        string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the filter.
func (s *FilterSpec) Name() string { return "trigger" }

// Requirements returns the requirements of the command.
func (s *FilterSpec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the trigger command.
func (s *FilterSpec) Exec(ctx context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	cloudFilters, err := status.ParseFilters(
		s.StatusFilters.StackStatus,
		s.StatusFilters.DeploymentStatus,
		s.StatusFilters.DriftStatus,
	)
	if err != nil {
		return err
	}

	report, err := s.engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, cloudFilters, false)
	if err != nil {
		return err
	}

	filteredStacks, err := s.engine.FilterStacks(report.Stacks,
		engine.ByWorkingDir(),
		engine.ByTags(s.Tags, s.NoTags),
	)
	if err != nil {
		return err
	}

	for _, st := range filteredStacks {
		stackTrigger := PathSpec{

			Change:       s.Change,
			IgnoreChange: s.IgnoreChange,
			Reason:       s.Reason,
			Path:         st.Stack.Dir.String(),
		}

		err := stackTrigger.Exec(ctx, cli)
		if err != nil {
			return err
		}
	}
	return nil
}
