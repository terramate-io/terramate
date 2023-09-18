// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package github implements a client SDK for the Github API.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

const (
	// ErrNotFound indicates the resource does not exists.
	ErrNotFound errors.Kind = "resource not found (HTTP Status: 404)"
	// ErrUnprocessableEntity indicates the entity cannot be processed for any reason.
	ErrUnprocessableEntity errors.Kind = "entity cannot be processed (HTTP Status: 422)"
)

const (
	// Domain is the default GitHub domain.
	Domain = "github.com"
	// APIDomain is the default GitHub API domain.
	APIDomain = "api." + Domain
	// APIBaseURL is the default base url for the GitHub API.
	APIBaseURL = "https://" + APIDomain
)

type (
	// Client is a Github HTTP client wrapper.
	Client struct {
		// BaseURL is the base URL used to construct the final URL of endpoints.
		// If not set, then api.github.com is used.
		BaseURL string

		// HTTPClient sets the HTTP client used and then allows for advanced
		// connection reuse schemes. If not set, a new http.Client is used.
		HTTPClient *http.Client

		// Token is the Github token (usually provided by the GH_TOKEN environment
		// variable.
		Token string
	}

	// OIDCVars is the variables used for issuing new OIDC tokens.
	OIDCVars struct {
		ReqURL   string
		ReqToken string
	}
)

// PullsForCommit returns a list of pull request objects associated with the
// given commit SHA.
func (c *Client) PullsForCommit(ctx context.Context, repository, commit string) (pulls []Pull, err error) {
	if !strings.Contains(repository, "/") {
		return nil, errors.E("expects a valid Github repository of format <owner>/<name>")
	}

	url := fmt.Sprintf("%s/repos/%s/commits/%s/pulls", c.baseURL(), repository, commit)
	data, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &pulls)
	if err != nil {
		return nil, errors.E(err, "unmarshaling pull list")
	}
	return pulls, nil
}

// Commit retrieves information about an specific commit in the GitHub API.
func (c *Client) Commit(ctx context.Context, repository, sha string) (*Commit, error) {
	if !strings.Contains(repository, "/") {
		return nil, errors.E("expects a valid Github repository of format <owner>/<name>")
	}
	url := fmt.Sprintf("%s/repos/%s/commits/%s", c.baseURL(), repository, sha)
	data, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}
	var commit Commit
	err = json.Unmarshal(data, &commit)
	if err != nil {
		return nil, errors.E(err, "unmarshaling commit info")
	}
	return &commit, nil
}

// OIDCToken requests a new OIDC token.
func (c *Client) OIDCToken(ctx context.Context, cfg OIDCVars) (token string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ReqURL, nil)
	if err != nil {
		return "", errors.E(err, "creating Github OIDC request")
	}

	req.Header.Set("Authorization", "Bearer "+cfg.ReqToken)

	data, err := c.doGetWithReq(req)
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

func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.E(err, "creating pulls request")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.doGetWithReq(req)
}

func (c *Client) doGetWithReq(req *http.Request) ([]byte, error) {
	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.E(err, "requesting GET %s", req.URL)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.E(err, "reading response body")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.E(ErrNotFound, "retrieving %s", req.URL)
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, errors.E(ErrUnprocessableEntity, "retrieving %s", req.URL)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.E("unexpected status code: %s while getting %s", resp.Status, req.URL)
	}
	return data, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		c.BaseURL = APIBaseURL
	}
	return c.BaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	return c.HTTPClient
}
