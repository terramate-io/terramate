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

		before := []string{}
		if len(stack.Before) > 0 {
			before = stack.Before
		}
		beforeVal, _ := json.Marshal(before)

		after := []string{}
		if len(stack.After) > 0 {
			after = stack.After
		}
		afterVal, _ := json.Marshal(after)

		wants := []string{}
		if len(stack.Wants) > 0 {
			wants = stack.Wants
		}
		wantsVal, _ := json.Marshal(wants)

		wantedBy := []string{}
		if len(stack.WantedBy) > 0 {
			wantedBy = stack.WantedBy
		}
		wantedByVal, _ := json.Marshal(wantedBy)

		s.printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stack.Dir))
		if stack.ID != "" {
			s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.id=%q", stack.ID))
		}
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.name=%q", stack.Name))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.description=%q", stack.Description))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.tags=%s", string(tagsVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.before=%s", string(beforeVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.after=%s", string(afterVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.wants=%s", string(wantsVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.wanted_by=%s", string(wantedByVal)))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.absolute=%q", stack.Dir))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.basename=%q", stack.PathBase()))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.relative=%q", stack.RelPath()))
		s.printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.to_root=%q", stack.RelPathToRoot(cfg)))
	}
	return nil
}
