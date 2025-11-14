// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package metadata provides the show-metadata command.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
)

// Spec is the command specification for the show-metadata command.
type Spec struct {
	GitFilter     engine.GitFilter
	StatusFilters resources.StatusFilters
	Tags          []string
	NoTags        []string

	engine   *engine.Engine
	printers printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug show metadata" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the show-metadata command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	logger := log.With().
		Str("action", "cli.printMetadata()").
		Logger()

	report, err := s.engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, s.StatusFilters, false)
	if err != nil {
		return errors.E(err, "loading metadata: listing stacks")
	}

	stackEntries, err := s.engine.FilterStacks(report.Stacks, engine.ByWorkingDir(), engine.ByTags(s.Tags, s.NoTags))
	if err != nil {
		return err
	}
	if len(stackEntries) == 0 {
		return nil
	}

	cfg := s.engine.Config()
	s.printers.Stdout.Println("Available metadata:")

	for _, stackEntry := range stackEntries {
		stack := stackEntry.Stack

		logger.Debug().
			Stringer("stack", stack).
			Msg("Print metadata for individual stack.")

		tags := []string{}
		if len(stack.Tags) > 0 {
			tags = stack.Tags
		}
		tagsVal, _ := json.Marshal(tags)

		s.printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stack.Dir))
		if stack.ID != "" {
			s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.id=%q", stack.ID))
		}
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.name=%q", stack.Name))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.description=%q", stack.Description))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.tags=%s", string(tagsVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.absolute=%q", stack.Dir))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.basename=%q", stack.PathBase()))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.relative=%q", stack.RelPath()))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.to_root=%q", stack.RelPathToRoot(cfg)))
	}
	return nil
}
