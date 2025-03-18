// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
	"github.com/terramate-io/terramate/printer"
)

type GoogleSpec struct {
	Printers  printer.Printers
	CliCfg    cliconfig.Config
	Verbosity int
}

func (s *GoogleSpec) Name() string { return "google login" }

func (s *GoogleSpec) Exec(ctx context.Context) error {
	err := auth.GoogleLogin(s.Printers, s.Verbosity, s.CliCfg)
	if err == nil {
		s.Printers.Stdout.Println("authenticated successfully")
	}
	return err
}
