// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package metadata provides the show-metadata command.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"

	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
)

// Spec is the command specification for the show-metadata command.
type Spec struct {
	WorkingDir    string
	Engine        *engine.Engine
	Printers      printer.Printers
	GitFilter     engine.GitFilter
	StatusFilters resources.StatusFilters
	Tags          []string
	NoTags        []string
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "debug show metadata" }

// Exec executes the show-metadata command.
func (s *Spec) Exec(_ context.Context) error {
	logger := log.With().
		Str("action", "cli.printMetadata()").
		Logger()

	report, err := s.Engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, s.StatusFilters, false)
	if err != nil {
		return errors.E(err, "loading metadata: listing stacks")
	}

	stackEntries, err := s.Engine.FilterStacks(report.Stacks, engine.ByWorkingDir(), engine.ByTags(s.Tags, s.NoTags))
	if err != nil {
		return err
	}
	if len(stackEntries) == 0 {
		return nil
	}

	cfg := s.Engine.Config()
	s.Printers.Stdout.Println("Available metadata:")

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

		s.Printers.Stdout.Println(fmt.Sprintf("\nstack %q:", stack.Dir))
		if stack.ID != "" {
			s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.id=%q", stack.ID))
		}
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.name=%q", stack.Name))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.description=%q", stack.Description))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.tags=%s", string(tagsVal)))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.absolute=%q", stack.Dir))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.basename=%q", stack.PathBase()))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.relative=%q", stack.RelPath()))
		s.Printers.Stdout.Println(fmt.Sprintf("\tterramate.stack.path.to_root=%q", stack.RelPathToRoot(cfg)))
	}
	return nil
}
