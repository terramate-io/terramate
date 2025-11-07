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
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/printer"
)

// Spec is the command specification for the debug globals command.
type Spec struct {
	GitFilter     engine.GitFilter
	StatusFilters resources.StatusFilters
	Tags          []string
	NoTags        []string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug globals" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the debug globals command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	report, err := s.engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, s.StatusFilters, false)
	if err != nil {
		return err
	}
	cfg := s.engine.Config()

	filteredStacks, err := s.engine.FilterStacks(report.Stacks, engine.ByWorkingDir(), engine.ByTags(s.Tags, s.NoTags))
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

		s.printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stack.Dir))
		for _, line := range strings.Split(globalsStrRepr, "\n") {
			s.printers.Stdout.Println(fmt.Sprintf("\t%s", line))
		}
	}
	return nil
}
