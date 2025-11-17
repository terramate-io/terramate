// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package version provides the version command.
package version

import (
	"context"
	"fmt"
	"time"

	"github.com/terramate-io/go-checkpoint"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

// Spec represents the version command specification.
type Spec struct {
	Product       string
	PrettyProduct string
	Version       string
	Full          bool

	InfoChan chan *checkpoint.CheckResponse
}

// Name returns the name of the version command.
func (s *Spec) Name() string { return "version" }

// Exec executes the version command.
func (s *Spec) Exec(ctx context.Context) error {
	if s.Full {
		fmt.Printf("%s %s\n", s.Product, s.Version)
	} else {
		fmt.Println(s.Version)
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
				s.PrettyProduct,
				info.CurrentVersion, releaseDate.Format(time.UnixDate),
				info.CurrentDownloadURL))
		}

		if len(info.Alerts) > 0 {
			plural := ""
			if len(info.Alerts) > 1 {
				plural = "s"
			}

			printer.Stdout.Println(fmt.Sprintf("\nYour version of %s has %d alert%s:\n", s.PrettyProduct, len(info.Alerts), plural))

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
