// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const apiKeyEnvName = "TMC_TOKEN"

// APIKey implements the credential interface.
type APIKey struct {
	token string

	orgs cloud.MemberOrganizations

	output out.O
	client *cloud.Client
}

func newAPIKey(output out.O, client *cloud.Client) *APIKey {
	return &APIKey{
		output: output,
		client: client,
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

	a.client.Credential = a

	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	orgs, err := a.client.MemberOrganizations(ctx)
	if err != nil {
		return true, err
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

// info display the credential details.
func (a *APIKey) info(selectedOrgName string) {
	printer.Stdout.Println("status: signed in")
	printer.Stdout.Println(fmt.Sprintf("provider: %s", a.Name()))

	if len(a.orgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("organizations: %s", a.orgs))
	}

	if selectedOrgName == "" && len(a.orgs) > 1 {
		printer.Stderr.Warn("User is member of multiple organizations but none was selected")
	}
}

func (a *APIKey) organizations() cloud.MemberOrganizations {
	return a.orgs
}
