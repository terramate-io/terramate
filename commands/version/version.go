// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package version provides the version command.
package version

import (
	"context"
	"fmt"
	"time"

	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

// Spec represents the version command specification.
type Spec struct {
	InfoChan chan *checkpoint.CheckResponse
}

// Name returns the name of the version command.
func (s *Spec) Name() string { return "version" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the version command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	// TODO(snk): Using the <product> <version> output would be a breaking change.
	// We change this separately later.
	if cli.Product() != "terramate" {
		fmt.Printf("%s %s\n", cli.Product(), cli.Version())
	} else {
		fmt.Println(cli.Version())
	}

	if s.InfoChan == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return errors.E(ctx.Err(), "waiting for checkpoint API response")
	case info := <-s.InfoChan:
		if info == nil {
			return nil
		}

		if info.Outdated {
			releaseDate := time.Unix(int64(info.CurrentReleaseDate), 0).UTC()
			printer.Stdout.Println(fmt.Sprintf("\nYour version of %s is out of date! The latest version\n"+
				"is %s (released on %s).\nYou can update by downloading from %s",
				cli.PrettyProduct(),
				info.CurrentVersion, releaseDate.Format(time.UnixDate),
				info.CurrentDownloadURL))
		}

		if len(info.Alerts) > 0 {
			plural := ""
			if len(info.Alerts) > 1 {
				plural = "s"
			}

			printer.Stdout.Println(fmt.Sprintf("\nYour version of %s has %d alert%s:\n", cli.PrettyProduct(), len(info.Alerts), plural))

			for _, alert := range info.Alerts {
				urlDesc := ""
				if alert.URL != "" {
					urlDesc = fmt.Sprintf(" (more information at %s)", alert.URL)
				}
				printer.Stdout.Println(fmt.Sprintf("\t- [%s] %s%s", alert.Level, alert.Message, urlDesc))
			}
		}
	}
	return nil
}
