// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package trigger

import (
	"context"

	"github.com/terramate-io/terramate/config/filter"
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
	WorkingDir    string
	Engine        *engine.Engine
	Printers      printer.Printers
	GitFilter     engine.GitFilter
	StatusFilters StatusFilters
	Change        bool
	IgnoreChange  bool
	Reason        string
}

// Name returns the name of the filter.
func (s *FilterSpec) Name() string { return "trigger" }

// Exec executes the trigger command.
func (s *FilterSpec) Exec(_ context.Context) error {
	cloudFilters, err := status.ParseFilters(
		s.StatusFilters.StackStatus,
		s.StatusFilters.DeploymentStatus,
		s.StatusFilters.DriftStatus,
	)
	if err != nil {
		return err
	}

	report, err := s.Engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, cloudFilters, false)
	if err != nil {
		return err
	}

	for _, st := range s.Engine.FilterStacks(report.Stacks, filter.TagClause{}) {
		stackTrigger := PathSpec{
			WorkingDir:   s.WorkingDir,
			Printers:     s.Printers,
			Engine:       s.Engine,
			Change:       s.Change,
			IgnoreChange: s.IgnoreChange,
			Reason:       s.Reason,
			Path:         st.Stack.Dir.String(),
		}

		err := stackTrigger.Exec(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}
