// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// GoogleSpec is the command specification for the google login command.
type GoogleSpec struct {
	Printers  printer.Printers
	CliCfg    cliconfig.Config
	Verbosity int
}

// Name returns the name of the command.
func (s *GoogleSpec) Name() string { return "google login" }

// Exec executes the google login command.
func (s *GoogleSpec) Exec(_ context.Context) error {
	err := cliauth.GoogleLogin(s.Printers, s.Verbosity, s.CliCfg)
	if err == nil {
		s.Printers.Stdout.Println("authenticated successfully")
	}
	return err
}
