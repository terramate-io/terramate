// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package runtimeenv provides the show-runtime-env command.
package runtimeenv

import (
	"context"
	"fmt"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/run"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
)

// Spec is the command specification for the show-runtime-env command.
type Spec struct {
	GitFilter     engine.GitFilter
	StatusFilters resources.StatusFilters
	Tags          []string
	NoTags        []string

	engine   *engine.Engine
	printers printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug show runtime-env" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any {
	return commands.RequireEngine()
}

// Exec executes the show-runtime-env command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	report, err := s.engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, s.StatusFilters, false)
	if err != nil {
		return errors.E(err, "listing stacks")
	}

	stackEntries, err := s.engine.FilterStacks(report.Stacks, engine.ByWorkingDir(), engine.ByTags(s.Tags, s.NoTags))
	if err != nil {
		return err
	}

	cfg := s.engine.Config()
	for _, stackEntry := range stackEntries {
		envVars, err := run.LoadEnv(cfg, stackEntry.Stack)
		if err != nil {
			return errors.E(err, "loading stack run environment")
		}

		s.printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stackEntry.Stack.Dir))

		for _, envVar := range envVars {
			s.printers.Stdout.Println(fmt.Sprintf("\t%s", envVar))
		}
	}
	return nil
}
