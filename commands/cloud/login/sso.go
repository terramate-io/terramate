// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"context"
	"fmt"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/ui/tui/cliauth"

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

// SSOSpec represents the login sso command specification.
type SSOSpec struct {
	Engine    *engine.Engine
	OrgName   string
	Printers  printer.Printers
	Region    cloud.Region
	Verbosity int
}

// Name returns the name of the login sso command.
func (s *SSOSpec) Name() string { return "login sso" }

// Exec executes the login sso command.
func (s *SSOSpec) Exec(_ context.Context) error {
	cloudURL, envFound := cliauth.EnvBaseURL()
	if !envFound {
		cloudURL = cloud.BaseURL(s.Region)
	}

	opts := cloud.Options{
		cloud.WithRegion(s.Region),
		cloud.WithHTTPClient(&s.Engine.HTTPClient),
	}
	if envFound {
		opts = append(opts, cloud.WithBaseURL(cloudURL))
	}

	client := cloud.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	ssoOrgID, err := client.GetOrgSingleSignOnID(ctx, s.OrgName)
	if err != nil {
		return errors.E("Organization %s doesn't have SSO enabled", s.OrgName)
	}

	err = cliauth.SSOLogin(printer.DefaultPrinters, s.Verbosity, ssoOrgID, s.Engine.CLIConfig())
	if err != nil {
		return errors.E(err, "Failed to authenticate")
	}

	err = s.Engine.LoadCredential("oidc.workos")
	if err != nil {
		return errors.E(err, "failed to load credentials")
	}

	ctx, cancel = context.WithTimeout(context.Background(), cloud.DefaultTimeout)
	defer cancel()
	client = s.Engine.CloudClient()
	user, err := client.Users(ctx)
	if err != nil {
		return errors.E(err, "failed to test token")
	}

	s.Printers.Stdout.Println(fmt.Sprintf("Logged in as %s", user.DisplayName))
	if s.Verbosity > 0 {
		s.Printers.Stdout.Println(fmt.Sprintf("Expire at: %s", s.Engine.Credential().ExpireAt().Format(time.RFC822Z)))
	}
	return nil
}
