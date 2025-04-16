// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/integrations/github"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const (
	defaultGithubTimeout   = 60 * time.Second
	githubOIDCProviderName = "GitHub Actions OIDC"
)

type githubOIDC struct {
	mu        sync.RWMutex
	token     string
	jwtClaims jwt.MapClaims

	expireAt  time.Time
	repoOwner string
	repoName  string

	reqURL   string
	reqToken string
	orgs     resources.MemberOrganizations

	printers  printer.Printers
	verbosity int
	client    *cloud.Client
}

func newGithubOIDC(printers printer.Printers, verbosity int, client *cloud.Client) *githubOIDC {
	return &githubOIDC{
		client:    client,
		printers:  printers,
		verbosity: verbosity,
	}
}

func (g *githubOIDC) Load() (bool, error) {
	const envReqURL = "ACTIONS_ID_TOKEN_REQUEST_URL"
	const envReqTok = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"

	g.reqURL = os.Getenv(envReqURL)
	if g.reqURL == "" {
		return false, nil
	}

	g.reqToken = os.Getenv(envReqTok)

	audience := oidcAudience(g.client.Region())
	if audience != "" {
		u, err := url.Parse(g.reqURL)
		if err != nil {
			return false, errors.E(err, "invalid ACTIONS_ID_TOKEN_REQUEST_URL env var")
		}

		qr := u.Query()
		qr.Set("audience", audience)
		u.RawQuery = qr.Encode()
		g.reqURL = u.String()
	}

	err := g.Refresh()
	if err != nil {
		return false, err
	}
	g.client.SetCredential(g)
	return true, g.fetchDetails()
}

func (g *githubOIDC) Name() string {
	return githubOIDCProviderName
}

func (g *githubOIDC) HasExpiration() bool {
	return true
}

func (g *githubOIDC) IsExpired() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return time.Now().After(g.expireAt)
}

func (g *githubOIDC) ExpireAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.expireAt
}

func (g *githubOIDC) Refresh() (err error) {
	if g.token != "" {
		if g.verbosity > 0 {
			g.printers.Stdout.Println("refreshing token...")
		}

		defer func() {
			if err == nil {
				if g.verbosity > 0 {
					g.printers.Stdout.Println("token successfully refreshed.")
					g.printers.Stdout.Println(fmt.Sprintf("next token refresh in: %s", time.Until(g.ExpireAt())))
				}
			}
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	token, err := github.OIDCToken(ctx, github.OIDCVars{
		ReqURL:   g.reqURL,
		ReqToken: g.reqToken,
	})

	if err != nil {
		return errors.E(err, "requesting new Github OIDC token")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.token = token
	g.jwtClaims, err = tokenClaims(g.token)
	if err != nil {
		return err
	}
	exp, ok := g.jwtClaims["exp"].(float64)
	if !ok {
		return errors.E(`cached JWT token has no "exp" field`)
	}
	sec, dec := math.Modf(exp)
	g.expireAt = time.Unix(int64(sec), int64(dec*(1e9)))

	repoOwner, ok := g.jwtClaims["repository_owner"].(string)
	if !ok {
		return errors.E(`GitHub OIDC JWT with no "repository_owner" payload field.`)
	}
	repoName, ok := g.jwtClaims["repository"].(string)
	if !ok {
		return errors.E(`GitHub OIDC JWT with no "repository" payload field.`)
	}
	g.repoOwner = repoOwner
	g.repoName = repoName
	return nil
}

func (g *githubOIDC) Claims() jwt.MapClaims {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.jwtClaims
}

func (g *githubOIDC) DisplayClaims() []keyValue {
	return []keyValue{
		{
			key:   "owner",
			value: g.repoOwner,
		},
		{
			key:   "repository",
			value: g.repoName,
		},
	}
}

func (g *githubOIDC) Token() (string, error) {
	if g.IsExpired() {
		err := g.Refresh()
		if err != nil {
			return "", err
		}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token, nil
}

func (g *githubOIDC) ApplyCredentials(req *http.Request) error {
	return applyJWTBasedCredentials(req, g)
}

func (g *githubOIDC) RedactCredentials(req *http.Request) {
	redactJWTBasedCredentials(req)
}

// Validate if the credential is ready to be used.
func (g *githubOIDC) fetchDetails() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloudTimeout)
	defer cancel()
	orgs, err := g.client.MemberOrganizations(ctx)
	if err != nil {
		return err
	}
	g.orgs = orgs
	return nil
}

func (g *githubOIDC) Info(selectedOrgName string) {
	printer.Stdout.Println(fmt.Sprintf("provider: %s", g.Name()))
	if selectedOrgName == "" {
		printer.Stderr.ErrorWithDetails(
			"Missing cloud configuration",
			errors.E("Please set TM_CLOUD_ORGANIZATION environment variable or "+
				"terramate.config.cloud.organization configuration attribute to a specific organization",
			),
		)
		return
	}
	trustedOrgs := g.orgs.TrustedOrgs()
	org, found := trustedOrgs.LookupByName(selectedOrgName)
	if !found {
		printer.Stdout.Println("status: untrusted")
		printer.Stderr.Error(errors.E("selected organization %s not found among trusted organizations", selectedOrgName))
		return
	}

	printer.Stdout.Println("status: trusted")
	printer.Stdout.Println(fmt.Sprintf("selected organization: %s", org))

	for _, kv := range g.DisplayClaims() {
		printer.Stdout.Println(fmt.Sprintf("%s: %s", kv.key, kv.value))
	}
}

// Organizations that the GitHub OIDC token belong to.
func (g *githubOIDC) Organizations() resources.MemberOrganizations {
	return g.orgs
}
