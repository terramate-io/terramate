// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const apiKeyEnvName = "TMC_TOKEN"

// APIKey implements the credential interface.
type APIKey struct {
	token string

	orgs   resources.MemberOrganizations
	client *cloud.Client

	printers  printer.Printers
	verbosity int
}

func newAPIKey(printers printer.Printers, verbosity int, client *cloud.Client) *APIKey {
	return &APIKey{
		client:    client,
		printers:  printers,
		verbosity: verbosity,
	}
}

// Name returns the name of the authentication method.
func (a *APIKey) Name() string {
	return "API Key"
}

// Load loads the API key from the environment.
func (a *APIKey) Load() (bool, error) {
	a.token = os.Getenv(apiKeyEnvName)
	if a.token == "" {
		return false, nil
	}

	a.client.SetCredential(a)

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	orgs, err := a.client.MemberOrganizations(ctx)
	if err != nil {
		return true, err
	}
	if a.verbosity > 0 {
		a.printers.Stdout.Println(fmt.Sprintf("API key loaded from %s environment variable", apiKeyEnvName))
	}
	a.orgs = orgs
	return true, nil
}

// Token returns the API key token.
func (a *APIKey) Token() (string, error) {
	return a.token, nil // never expires
}

// ApplyCredentials applies the API key to the request.
func (a *APIKey) ApplyCredentials(req *http.Request) error {
	req.SetBasicAuth(a.token, "")
	return nil
}

// RedactCredentials redacts the API key from the request.
func (a *APIKey) RedactCredentials(req *http.Request) {
	req.SetBasicAuth("REDACTED", "")
}

// HasExpiration returns false because the CLI has no information about the expiration of the API key.
func (a *APIKey) HasExpiration() bool { return false }

// IsExpired returns false because the CLI has no information about the expiration of the API key.
func (a *APIKey) IsExpired() bool {
	return false
}

// ExpireAt should never be called.
func (a *APIKey) ExpireAt() time.Time {
	panic(errors.E(errors.ErrInternal, "API key does not expire"))
}

// Info display the credential details.
func (a *APIKey) Info(selectedOrgName string) {
	printer.Stdout.Println("status: signed in")
	printer.Stdout.Println(fmt.Sprintf("provider: %s", a.Name()))

	activeOrgs := a.orgs.ActiveOrgs()
	if len(activeOrgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("active organizations: %s", activeOrgs))
	}

	if len(activeOrgs) == 0 {
		printer.Stderr.Warnf("You are not part of an organization. Please join an organization or visit %s to create a new one.", cloud.HTMLURL(a.client.Region()))
	}

	if selectedOrgName == "" {
		printer.Stderr.ErrorWithDetails(
			"Missing cloud configuration",
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
				"terramate.config.cloud.organization configuration attribute to a specific organization",
			),
		)
		return
	}

	org, found := activeOrgs.LookupByName(selectedOrgName)
	if found {
		printer.Stdout.Println(fmt.Sprintf("selected organization: %s", org))
	} else {
		printer.Stderr.Error(errors.E("selected organization %q not found in the list of active organizations", selectedOrgName))
	}
}

// Organizations that the API key belong to.
func (a *APIKey) Organizations() resources.MemberOrganizations {
	return a.orgs
}
