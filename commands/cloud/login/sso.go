// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"
	"fmt"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

// SSOSpec represents the login sso command specification.
type SSOSpec struct {
	Verbosity int

	engine   *engine.Engine
	printers printer.Printers
	cliCfg   cliconfig.Config
}

// Name returns the name of the login sso command.
func (s *SSOSpec) Name() string { return "login sso" }

// Requirements returns the requirements of the command.
func (s *SSOSpec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the login sso command.
func (s *SSOSpec) Exec(_ context.Context, cli commands.CLI) error {
	s.engine = cli.Engine()
	s.printers = cli.Printers()
	s.cliCfg = cli.Config()

	orgName := s.engine.CloudOrgName()
	region := s.engine.CloudRegion()

	if orgName == "" {
		return errors.E(
			errors.E("No Terramate Cloud organization configured."),
			"Set `terramate.config.cloud.organization` or export `TM_CLOUD_ORGANIZATION` to the organization shortname that you intend to login.",
		)
	}

	cloudURL, envFound := cliauth.EnvBaseURL()
	if !envFound {
		cloudURL = cloud.BaseURL(region)
	}

	opts := cloud.Options{
		cloud.WithRegion(region),
		cloud.WithHTTPClient(&s.engine.HTTPClient),
	}
	if envFound {
		opts = append(opts, cloud.WithBaseURL(cloudURL))
	}

	client := cloud.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	ssoOrgID, err := client.GetOrgSingleSignOnID(ctx, orgName)
	if err != nil {
		return errors.E("Organization %s doesn't have SSO enabled", orgName)
	}

	err = cliauth.SSOLogin(printer.DefaultPrinters, s.Verbosity, ssoOrgID, s.cliCfg)
	if err != nil {
		return errors.E(err, "Failed to authenticate")
	}

	err = s.engine.LoadCredential("oidc.workos")
	if err != nil {
		return errors.E(err, "failed to load credentials")
	}

	ctx, cancel = context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	client = s.engine.CloudClient()
	user, err := client.Users(ctx)
	if err != nil {
		return errors.E(err, "failed to test token")
	}

	s.printers.Stdout.Println(fmt.Sprintf("Logged in as %s", user.DisplayName))
	if s.Verbosity > 0 {
		s.printers.Stdout.Println(fmt.Sprintf("Expire at: %s", s.engine.Credential().ExpireAt().Format(time.RFC822Z)))
	}
	return nil
}
