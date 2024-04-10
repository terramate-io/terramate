// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/github"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const githubOIDCProviderName = "GitHub Actions OIDC"

type githubOIDC struct {
	mu        sync.RWMutex
	token     string
	jwtClaims jwt.MapClaims

	expireAt  time.Time
	repoOwner string
	repoName  string

	reqURL   string
	reqToken string
	orgs     cloud.MemberOrganizations

	output out.O
	client *cloud.Client
}

func newGithubOIDC(output out.O, client *cloud.Client) *githubOIDC {
	return &githubOIDC{
		output: output,
		client: client,
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

	audience := oidcAudience()
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
	g.client.Credential = g
	return true, g.fetchDetails()
}

func (g *githubOIDC) Name() string {
	return githubOIDCProviderName
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
		g.output.MsgStdOutV("refreshing token...")

		defer func() {
			if err == nil {
				g.output.MsgStdOutV("token successfully refreshed.")
				g.output.MsgStdOutV("next token refresh in: %s", time.Until(g.ExpireAt()))
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

// Validate if the credential is ready to be used.
func (g *githubOIDC) fetchDetails() error {
	const apiTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	orgs, err := g.client.MemberOrganizations(ctx)
	if err != nil {
		return err
	}
	g.orgs = orgs
	return nil
}

func (g *githubOIDC) info(selectedOrgName string) {
	if len(g.orgs) > 0 && g.orgs[0].Status == "trusted" {
		printer.Stdout.Println("status: signed in")
	} else {
		printer.Stdout.Println("status: untrusted")
	}

	printer.Stdout.Println(fmt.Sprintf("provider: %s", g.Name()))

	for _, kv := range g.DisplayClaims() {
		printer.Stdout.Println(fmt.Sprintf("%s: %s", kv.key, kv.value))
	}

	if len(g.orgs) > 0 {
		printer.Stdout.Println(fmt.Sprintf("organizations: %s", g.orgs))
	}

	if selectedOrgName == "" && len(g.orgs) > 1 {
		printer.Stderr.Warn("User is member of multiple organizations but none was selected")
	}

	if len(g.orgs) == 0 {
		printer.Stderr.Warn("You are not part of an organization. Please visit cloud.terramate.io to create an organization.")
	}
}

func (g *githubOIDC) organizations() cloud.MemberOrganizations {
	return g.orgs
}
