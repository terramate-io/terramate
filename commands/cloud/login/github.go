// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/tmcloud/auth"
	"github.com/terramate-io/terramate/printer"
)

type GithubSpec struct {
	Printers printer.Printers
	CliCfg   cliconfig.Config
}

func (s *GithubSpec) Name() string { return "github login" }

func (s *GithubSpec) Exec(ctx context.Context) error {
	tmcURL, foundEnv := tmcloud.EnvBaseURL()
	if !foundEnv {
		tmcURL = cloud.BaseURL(cloud.EU)
	}
	err := auth.GithubLogin(s.Printers, tmcURL, s.CliCfg)
	if err == nil {
		s.Printers.Stdout.Println("authenticated successfully")
	}
	return err
}
