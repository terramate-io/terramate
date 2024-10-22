// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const gitlabOIDCProviderName = "GitLab Actions OIDC"

type gitlabOIDC struct {
	mu        sync.RWMutex
	token     string
	jwtClaims jwt.MapClaims

	expireAt time.Time
	repoName string

	orgs cloud.MemberOrganizations

	output out.O
	client *cloud.Client
}

func newGitlabOIDC(output out.O, client *cloud.Client) *gitlabOIDC {
	return &gitlabOIDC{
		output: output,
		client: client,
	}
}

func (g *gitlabOIDC) Load() (bool, error) {
	const envToken = "TM_GITLAB_ID_TOKEN"

	g.token = os.Getenv(envToken)
	if g.token == "" {
		return false, nil
	}
	var err error
	g.jwtClaims, err = tokenClaims(g.token)
	if err != nil {
		return true, err
	}

	exp, ok := g.jwtClaims["exp"].(float64)
	if !ok {
		return true, errors.E(`JWT token has no "exp" field`)
	}
	sec, dec := math.Modf(exp)
	g.expireAt = time.Unix(int64(sec), int64(dec*(1e9)))

	repoName, ok := g.jwtClaims["project_path"].(string)
	if !ok {
		return true, errors.E(`Gitlab OIDC JWT with no "project_path" payload field.`)
	}
	g.repoName = repoName

	g.client.Credential = g
	return true, g.fetchDetails()
}

func (g *gitlabOIDC) Name() string {
	return gitlabOIDCProviderName
}

func (g *gitlabOIDC) IsExpired() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return time.Now().After(g.expireAt)
}

func (g *gitlabOIDC) ExpireAt() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.expireAt
}

func (g *gitlabOIDC) Claims() jwt.MapClaims {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.jwtClaims
}

func (g *gitlabOIDC) DisplayClaims() []keyValue {
	return []keyValue{
		{
			key:   "repository",
			value: g.repoName,
		},
	}
}

func (g *gitlabOIDC) Token() (string, error) {
	if g.IsExpired() {
		return "", errors.E("GitLab OIDC token is expired. Please increase the job timeout.")
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token, nil
}

// Validate if the credential is ready to be used.
func (g *gitlabOIDC) fetchDetails() error {
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

func (g *gitlabOIDC) info(selectedOrgName string) {
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

func (g *gitlabOIDC) organizations() cloud.MemberOrganizations {
	return g.orgs
}
