// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
)

// ErrOnboardingIncomplete indicates the onboarding process is incomplete.
const ErrOnboardingIncomplete errors.Kind = "cloud commands cannot be used until onboarding is complete"

type cloudConfig struct {
	baseAPI string
	client  *http.Client
	output  out.O

	credential credential
}

type credential interface {
	Name() string
	Load() (bool, error)
	Token() (string, error)
	Refresh() error
	Claims() jwt.MapClaims
	IsExpired() bool
	ExpireAt() time.Time
	Validate(cloudcfg cloudConfig) error
	Info()
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

func (c *cli) checkSyncDeployment() {
	err := c.setupSyncDeployment()
	if err != nil {
		if errors.IsKind(err, ErrOnboardingIncomplete) {
			c.cred().Info()
		}
		fatal(err)
	}
}

func (c *cli) setupSyncDeployment() error {
	cred, err := c.loadCredential()
	if err != nil {
		return err
	}

	c.cloud = cloudConfig{
		baseAPI:    cloudBaseURL,
		client:     &http.Client{},
		output:     c.output,
		credential: cred,
	}

	return cred.Validate(c.cloud)
}

func (c *cli) cloudInfo() {
	err := c.setupSyncDeployment()
	if err != nil {
		fatal(err)
	}
	c.cred().Info()
	// verbose info
	c.cloud.output.MsgStdOutV("next token refresh in: %s", time.Until(c.cred().ExpireAt()))
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
