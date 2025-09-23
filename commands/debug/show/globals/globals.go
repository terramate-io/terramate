// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package globals provides the debug globals command.
package globals

import (
	"context"
	"fmt"
	"strings"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/printer"
)

// Spec is the command specification for the debug globals command.
type Spec struct {
	WorkingDir string
	Engine     *engine.Engine
	Printers   printer.Printers
	GitFilter  engine.GitFilter
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug globals" }

// Exec executes the debug globals command.
func (s *Spec) Exec(_ context.Context) error {
	report, err := s.Engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, resources.NoStatusFilters(), false)
	if err != nil {
		return err
	}
	cfg := s.Engine.Config()

	filteredStacks, err := s.Engine.FilterStacks(report.Stacks, engine.ByWorkingDir())
	if err != nil {
		return err
	}

	for _, stackEntry := range filteredStacks {
		stack := stackEntry.Stack
		report := globals.ForStack(cfg, stack)
		if err := report.AsError(); err != nil {
			return errors.E(err, "listing stacks globals: loading stack at %s", stack.Dir)
		}

		globalsStrRepr := report.Globals.String()
		if globalsStrRepr == "" {
			continue
		}

		s.Printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stack.Dir))
		for _, line := range strings.Split(globalsStrRepr, "\n") {
			s.Printers.Stdout.Println(fmt.Sprintf("\t%s", line))
		}
	}
	return nil
}
