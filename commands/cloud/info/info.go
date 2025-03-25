// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package info provides the cloud info command.
package info

import (
	"context"
	"fmt"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/errors/verbosity"
	"github.com/terramate-io/terramate/printer"
	auth "github.com/terramate-io/terramate/ui/tui/cliauth"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

// Spec is the command specification for the cloud info command.
type Spec struct {
	Engine    *engine.Engine
	Printers  printer.Printers
	Verbosity int
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "cloud info" }

// Exec executes the cloud info command.
func (s *Spec) Exec(_ context.Context) error {
	err := s.Engine.LoadCredential()
	if err != nil {
		if errors.IsKind(err, auth.ErrLoginRequired) {
			return errors.E(
				newCloudLoginRequiredError([]string{"The `terramate cloud info` shows information about your current credentials to Terramate Cloud."}).WithCause(err),
				"failed to load the cloud credentials",
			)
		}
		if errors.IsKind(err, clitest.ErrCloudOnboardingIncomplete) {
			return newCloudOnboardingIncompleteError(s.Engine.CloudClient().Region())
		}
		return errors.E(err, "failed to load the cloud credentials")
	}
	cred := s.Engine.Credential()
	cred.Info(s.Engine.CloudOrgName())

	// verbose info
	if s.Verbosity > 0 && cred.HasExpiration() {
		if s.Verbosity > 0 {
			printer.Stdout.Println(fmt.Sprintf("next token refresh in: %s", time.Until(cred.ExpireAt())))
		}
	}
	return nil
}

// newCloudLoginRequiredError creates an error indicating that a cloud login is required to use requested features.
func newCloudLoginRequiredError(requestedFeatures []string) *errors.DetailedError {
	err := errors.D(clitest.CloudLoginRequiredMessage)

	for _, s := range requestedFeatures {
		err = err.WithDetailf(verbosity.V1, "%s", s)
	}

	err = err.WithDetailf(verbosity.V1, "To login with an existing account, run 'terramate cloud login'.").
		WithDetailf(verbosity.V1, "To create a free account, visit https://cloud.terramate.io.")

	return err.WithCode(clitest.ErrCloud)
}

func newCloudOnboardingIncompleteError(region cloud.Region) *errors.DetailedError {
	err := errors.D(clitest.CloudOnboardingIncompleteMessage)
	err = err.WithDetailf(verbosity.V1, "Visit %s to setup your account.", cloud.HTMLURL(region))
	return err.WithCode(clitest.ErrCloudOnboardingIncomplete)
}
