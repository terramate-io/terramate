// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package github implements a client SDK for the Github API.
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/terramate-io/terramate/errors"
)

type (
	// Client is a Github HTTP client wrapper.
	Client struct {
		// HTTPClient sets the HTTP client used and then allows for advanced
		// connection reuse schemes. If not set, a new http.Client is used.
		HTTPClient *http.Client
	}

	// OIDCVars is the variables used for issuing new OIDC tokens.
	OIDCVars struct {
		ReqURL   string
		ReqToken string
	}
)

// OIDCToken requests a new OIDC token.
func (c *Client) OIDCToken(ctx context.Context, cfg OIDCVars) (token string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ReqURL, nil)
	if err != nil {
		return "", errors.E(err, "creating Github OIDC request")
	}

	req.Header.Set("Authorization", "Bearer "+cfg.ReqToken)

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.E(err, "issuing GET %s", cfg.ReqURL)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.E(err, "reading Github OIDC response body")
	}

	type response struct {
		Value string `json:"value"`
	}

	var tokresp response
	err = json.Unmarshal(data, &tokresp)
	if err != nil {
		return "", errors.E(err, "unmarshaling Github OIDC JSON response")
	}

	return tokresp.Value, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	return c.HTTPClient
}
