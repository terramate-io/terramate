// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdjson "encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
	"github.com/terramate-io/terramate/errors"
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

	output out.O
}

func newGithubOIDC(output out.O) *githubOIDC {
	return &githubOIDC{
		output: output,
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
	return err == nil, err
}

func (g *githubOIDC) Name() string {
	return "GitHub Actions OIDC"
}

func (g *githubOIDC) IsExpired() bool {
	var empty time.Time
	if g.expireAt == empty {
		return false
	}
	return time.Now().After(g.expireAt)
}

func (g *githubOIDC) Refresh() error {
	const oidcTimeout = 3 // seconds
	ctx, cancel := context.WithTimeout(context.Background(), oidcTimeout*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", g.reqURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+g.reqToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			g.output.MsgStdErrV("failed to close response body: %v", err)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type response struct {
		Value string `json:"value"`
	}

	var tokresp response
	err = stdjson.Unmarshal(data, &tokresp)
	if err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.token = tokresp.Value
	g.jwtClaims, err = tokenClaims(g.token)
	if err != nil {
		return err
	}
	exp, ok := g.jwtClaims["exp"].(int64)
	if !ok {
		return errors.E("GitHub OIDC JWT token has no expiration field")
	}
	g.expireAt = time.Unix(exp, 0)
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

func (g *githubOIDC) String() string {
	return ""
}
