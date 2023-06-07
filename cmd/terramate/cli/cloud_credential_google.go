// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	stdjson "encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
)

type googleSocial struct {
	mu        sync.RWMutex
	token     string
	jwtClaims jwt.MapClaims

	expireAt time.Time
}

func newGoogleSocial() *googleSocial {
	return &googleSocial{}
}

func (g *googleSocial) Load() (bool, error) {

	err := g.Refresh()
	return err == nil, err
}

func (g *googleSocial) Name() string {
	return "GitHub Actions OIDC"
}

func (g *googleSocial) IsExpired() bool {
	var empty time.Time
	if g.expireAt == empty {
		return false
	}
	return time.Now().After(g.expireAt)
}

func (g *googleSocial) Refresh() error {
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

func (g *googleSocial) Token() (string, error) {
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

func (g *googleSocial) String() string {
	return ""
}
