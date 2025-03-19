// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/cloud"

	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

// GithubSpec is the command specification for the github login command.
type GithubSpec struct {
	Printers  printer.Printers
	CliCfg    cliconfig.Config
	Verbosity int
}

// Name returns the name of the command.
func (s *GithubSpec) Name() string { return "github login" }

// Exec executes the github login command.
func (s *GithubSpec) Exec(_ context.Context) error {
	tmcURL, foundEnv := cliauth.EnvBaseURL()
	if !foundEnv {
		tmcURL = cloud.BaseURL(cloud.EU)
	}
	err := cliauth.GithubLogin(s.Printers, s.Verbosity, tmcURL, s.CliCfg)
	if err == nil {
		s.Printers.Stdout.Println("authenticated successfully")
	}
	return err
}
