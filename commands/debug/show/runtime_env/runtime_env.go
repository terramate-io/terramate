// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package runtimeenv provides the show-runtime-env command.
package runtimeenv

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
)

// Spec is the command specification for the show-runtime-env command.
type Spec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	GitFilter  engine.GitFilter
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug show runtime-env" }

// Exec executes the show-runtime-env command.
func (s *Spec) Exec(_ context.Context) error {
	report, err := s.Engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, resources.NoStatusFilters(), false)
	if err != nil {
		return errors.E(err, "listing stacks")
	}

	stackEntries, err := s.Engine.FilterStacks(report.Stacks, engine.ByWorkingDir())
	if err != nil {
		return err
	}

	cfg := s.Engine.Config()
	for _, stackEntry := range stackEntries {
		envVars, err := run.LoadEnv(cfg, stackEntry.Stack)
		if err != nil {
			return errors.E(err, "loading stack run environment")
		}

		s.Printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stackEntry.Stack.Dir))

		for _, envVar := range envVars {
			s.Printers.Stdout.Println(fmt.Sprintf("\t%s", envVar))
		}
	}
	return nil
}
