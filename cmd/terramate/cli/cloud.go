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
	client := cloud.Client{
		BaseURL:    cloudBaseURL,
		Credential: cloudcfg.credential,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orgs, err := client.MemberOrganizations(ctx)
	if err != nil {
		return err
	}

	cloudcfg.output.MsgStdOut("status: signed in")
	cloudcfg.output.MsgStdOut("provider: %s", cloudcfg.credential.Name())

	for _, kv := range cloudcfg.credential.DisplayClaims() {
		cloudcfg.output.MsgStdOut("%s: %s", kv.key, kv.value)
	}

	cloudcfg.output.MsgStdOut("organizations: %s", orgs)

	// verbose info
	cloudcfg.output.MsgStdOutV("next token refresh in: %s", time.Until(cloudcfg.credential.ExpireAt()))
	return nil
}
