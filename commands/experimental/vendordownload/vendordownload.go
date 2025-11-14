// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package vendordownload provides the vendor download command.
package vendordownload

import (
	"context"
	"fmt"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
)

// Spec represents the vendor download specification.
type Spec struct {
	Dir       string
	Source    string
	Reference string

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the vendor download command.
func (s *Spec) Name() string { return "vendor" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the vendor download command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	source := s.Source
	ref := s.Reference

	rootdir := s.engine.Config().HostDir()
	logger := log.With().
		Str("workingDir", s.workingDir).
		Str("rootdir", rootdir).
		Str("action", "cli.vendor()").
		Str("source", source).
		Str("ref", ref).
		Logger()

	parsedSource, err := tf.ParseSource(source)
	if err != nil {
		return errors.E("parsing module source %s: %s", source, err)
	}
	if parsedSource.Ref != "" {
		return errors.E("module source %s should not contain a reference", source)
	}
	parsedSource.Ref = ref

	eventsStream := download.NewEventStream()
	eventsHandled := s.handleVendorProgressEvents(eventsStream)

	logger.Debug().Msg("vendoring")

	var vendorDir project.Path
	dir := s.Dir
	if dir != "" {
		if !path.IsAbs(dir) {
			dir = project.PrjAbsPath(rootdir, s.workingDir).Join(dir).String()
		}
		vendorDir = project.NewPath(dir)
	} else {
		vendorDir, err = s.engine.VendorDir()
		if err != nil {
			return errors.E(err, "failed to get vendor directory")
		}
	}

	report := download.Vendor(rootdir, vendorDir, parsedSource, eventsStream)

	logger.Debug().Msg("finished vendoring, waiting for all vendor events to be handled")

	close(eventsStream)
	<-eventsHandled

	logger.Debug().Msg("vendor events handled, creating final report")

	if report.Error != nil {
		if errs, ok := report.Error.(*errors.List); ok {
			for _, err := range errs.Errors() {
				logger.Error().Err(err).Send()
			}
		} else {
			logger.Error().Err(report.Error).Send()
		}
	}

	s.printers.Stdout.Println(report.String())
	return nil
}

func (s *Spec) handleVendorProgressEvents(eventsStream download.ProgressEventStream) <-chan struct{} {
	eventsHandled := make(chan struct{})

	go func() {
		for event := range eventsStream {
			s.printers.Stdout.Println(fmt.Sprintf("vendor: %s %s at %s",
				event.Message, event.Module.Raw, event.TargetDir))
			log.Info().
				Str("module", event.Module.Raw).
				Stringer("vendorDir", event.TargetDir).
				Msg(event.Message)
		}
		close(eventsHandled)
	}()

	return eventsHandled
}
