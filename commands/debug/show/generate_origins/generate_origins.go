// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package generateorigins provides the generate-origins command.
package generateorigins

import (
	"context"
	"fmt"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/cloud/api/resources"
	cloudstack "github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

// Spec is the command specification for the generate-origins command.
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
func (s *Spec) Name() string { return "debug generate-origins" }

// Exec executes the generate-origins command.
func (s *Spec) Exec(_ context.Context) error {
	report, err := s.Engine.ListStacks(s.GitFilter, cloudstack.AnyTarget, s.StatusFilters, false)
	if err != nil {
		return errors.E(err, "generate debug: selecting stacks")
	}

	filteredStacks, err := s.Engine.FilterStacks(report.Stacks, engine.ByWorkingDir(), engine.ByTags(s.Tags, s.NoTags))
	if err != nil {
		return err
	}

	selectedStacks := map[project.Path]struct{}{}
	for _, entry := range filteredStacks {
		selectedStacks[entry.Stack.Dir] = struct{}{}
	}

	vendorDir, err := s.Engine.VendorDir()
	if err != nil {
		return err
	}

	cfg := s.Engine.Config()
	results, err := generate.Load(cfg, vendorDir)
	if err != nil {
		return errors.E(err, "generate debug: loading generated code")
	}

	for _, res := range results {
		if _, ok := selectedStacks[res.Dir]; !ok {
			log.Debug().Msgf("discarding dir %s since it is not a selected stack", res.Dir)
			continue
		}
		if res.Err != nil {
			errmsg := fmt.Sprintf("generate debug error on dir %s: %v", res.Dir, res.Err)
			log.Error().Msg(errmsg)
			s.Printers.Stderr.Println(errmsg)
			continue
		}

		files := make([]generate.GenFile, 0, len(res.Files))
		for _, f := range res.Files {
			if f.Condition() {
				files = append(files, f)
			}
		}

		for _, file := range files {
			filepath := path.Join(res.Dir.String(), file.Label())
			s.Printers.Stdout.Println(fmt.Sprintf("%s origin: %v", filepath, file.Range()))
		}
	}
	return nil
}
