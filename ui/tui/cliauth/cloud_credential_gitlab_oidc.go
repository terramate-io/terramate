// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliauth

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/api/resources"
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

	orgs resources.MemberOrganizations

	client *cloud.Client
}

func newGitlabOIDC(client *cloud.Client) *gitlabOIDC {
	return &gitlabOIDC{
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

	g.client.SetCredential(g)
	return true, g.fetchDetails()
}

func (g *gitlabOIDC) HasExpiration() bool {
	return true
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

func (g *gitlabOIDC) ApplyCredentials(req *http.Request) error {
	return applyJWTBasedCredentials(req, g)
}

func (g *gitlabOIDC) RedactCredentials(req *http.Request) {
	redactJWTBasedCredentials(req)
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

// Info display the credential details.
func (g *gitlabOIDC) Info(selectedOrgName string) {
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

// Organizations that the GitLab OIDC belong to.
func (g *gitlabOIDC) Organizations() resources.MemberOrganizations {
	return g.orgs
}
