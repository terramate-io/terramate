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
	"github.com/terramate-io/terramate/errors"
)

type githubOIDC struct {
	mu        sync.RWMutex
	token     string
	jwtClaims jwt.MapClaims

	expireAt time.Time

	reqURL   string
	reqToken string
}

func newGithubOIDC() *githubOIDC {
	return &githubOIDC{}
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
		_ = resp.Body.Close()
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
	jwtParser := &jwt.Parser{}
	token, _, err := jwtParser.ParseUnverified(g.token, jwt.MapClaims{})
	if err != nil {
		return err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		g.jwtClaims = claims

		if exp, ok := claims["exp"]; ok {
			g.expireAt = time.Unix(exp.(int64), 0)
		}
	}

	return nil
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
