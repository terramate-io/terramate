// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// GoogleSpec is the command specification for the google login command.
type GoogleSpec struct {
	Verbosity int

	printers printer.Printers
	cliCfg   cliconfig.Config
}

// Name returns the name of the command.
func (s *GoogleSpec) Name() string { return "google login" }

// Requirements returns the requirements of the command.
func (s *GoogleSpec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the google login command.
func (s *GoogleSpec) Exec(_ context.Context, cli commands.CLI) error {
	s.printers = cli.Printers()
	s.cliCfg = cli.Config()

	err := cliauth.GoogleLogin(s.printers, s.Verbosity, s.cliCfg)
	if err == nil {
		s.printers.Stdout.Println("authenticated successfully")
	}
	return err
}
