// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/printer"

	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// GithubSpec is the command specification for the github login command.
type GithubSpec struct {
	Verbosity int

	printers printer.Printers
	cliCfg   cliconfig.Config
}

// Name returns the name of the command.
func (s *GithubSpec) Name() string { return "github login" }

// Requirements returns the requirements of the command.
func (s *GithubSpec) Requirements(context.Context, commands.CLI) any { return nil }

// Exec executes the github login command.
func (s *GithubSpec) Exec(_ context.Context, cli commands.CLI) error {
	s.printers = cli.Printers()
	s.cliCfg = cli.Config()

	tmcURL, foundEnv := cliauth.EnvBaseURL()
	if !foundEnv {
		tmcURL = cloud.BaseURL(cloud.EU)
	}
	err := cliauth.GithubLogin(s.printers, s.Verbosity, tmcURL, s.cliCfg)
	if err == nil {
		s.printers.Stdout.Println("authenticated successfully")
	}
	return err
}
