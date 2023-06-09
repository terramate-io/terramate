// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
)

type cloudConfig struct {
	baseAPI    string
	client     *http.Client
	credential credential

	output out.O
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	Claims() jwt.MapClaims
	DisplayClaims() []keyValue
	IsExpired() bool
	ExpireAt() time.Time
	String() string
}

type keyValue struct {
	key   string
	value string
}

func credentialPrecedence(output out.O, clicfg cliconfig.Config) []credential {
	return []credential{
		newGithubOIDC(output),
		newGoogleCredential(output, clicfg),
	}
}

func (c *cli) cloudInfo() {
	cred, err := c.loadCredential()
	if err != nil {
		fatal(err)
	}

	cloud := cloudConfig{
		baseAPI:    cloudBaseURL,
		credential: cred,
		client:     &http.Client{},
		output:     c.output,
	}

	err = cloud.Info()
	if err != nil {
		c.output.MsgStdErr("error: %v", err)
	}
}

func (c *cli) loadCredential() (credential, error) {
	probes := credentialPrecedence(c.output, c.clicfg)
	var cred credential
	var found bool
	for _, probe := range probes {
		var err error
		found, err = probe.Load()
		if err != nil {
			return nil, err
		}
		if found {
			cred = probe
			break
		}
	}
	if !found {
		return nil, errors.E("no credential found")
	}

	return cred, nil
}

func tokenClaims(token string) (jwt.MapClaims, error) {
	jwtParser := &jwt.Parser{}
	tokParsed, _, err := jwtParser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.E(err, "parsing jwt token")
	}

	if claims, ok := tokParsed.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.E("invalid jwt token claims")
}

func (cloudcfg *cloudConfig) Info() error {
	cred := cloudcfg.credential
	client := cloud.Client{
		BaseURL:    cloudBaseURL,
		Credential: cred,
	}

	const apiTimeout = 5 * time.Second

	var (
		err  error
		user cloud.User
		orgs cloud.MemberOrganizations
	)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		orgs, err = client.MemberOrganizations(ctx)
	}()

	if err != nil {
		return err
	}

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		user, err = client.Users(ctx)
	}()

	if err != nil && !errors.IsKind(err, cloud.ErrNotFound) {
		return err
	}

	userErr := err

	if len(orgs) == 0 && cred.Name() == githubOIDCProviderName {
		cloudcfg.output.MsgStdOut("status: untrusted")
	} else {
		cloudcfg.output.MsgStdOut("status: signed in")
	}

	cloudcfg.output.MsgStdOut("provider: %s", cred.Name())

	if userErr == nil && user.DisplayName != "" {
		cloudcfg.output.MsgStdOut("user: %s", user.DisplayName)
	}

	for _, kv := range cred.DisplayClaims() {
		cloudcfg.output.MsgStdOut("%s: %s", kv.key, kv.value)
	}

	if len(orgs) > 0 {
		cloudcfg.output.MsgStdOut("organizations: %s", orgs)
	}

	if user.DisplayName == "" {
		cloudcfg.output.MsgStdErr("Warning: On-boarding is incomplete.  Please visit cloud.terramte.io to complete on-boarding.")
	}

	if len(orgs) == 0 {
		cloudcfg.output.MsgStdErr("Warning: You are not part of an organization. Please visit cloud.terramate.io to create an organization.")
	}

	// verbose info
	cloudcfg.output.MsgStdOutV("next token refresh in: %s", time.Until(cred.ExpireAt()))
	return nil
}
