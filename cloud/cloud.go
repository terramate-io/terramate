// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package cloud implements a client SDK for communication with the cloud API.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/terramate-io/terramate/errors"
)

// Host of the official Terramate Cloud API.
const Host = "api.terramate.io"

// BaseURL is the default cloud.terramate.io base API URL.
const BaseURL = "https://" + Host + "/v1"

// ErrUnexpectedStatus indicates the server responded with an unexpected status code.
const ErrUnexpectedStatus errors.Kind = "unexpected status code"

// ErrNotFound indicates the requested resource does not exist in the server.
const ErrNotFound errors.Kind = "resource not found"

// ErrUnexpectedResponseBody indicates the server responded with an unexpected body.
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"

type (
	// Client is the cloud SDK client.
	Client struct {
		// BaseURL is the cloud base endpoint URL.
		// If not set, it defaults to [BaseURL].
		BaseURL    string
		Credential Credential

		// HTTPClient is the HTTP client reused in all connections.
		// if not set, a new instance of http.Client is created on the first request.
		HTTPClient *http.Client
	}

	// Credential is the interface for the credential providers.
	Credential interface {
		// Token retrieves a new token ready be used (the credential provider must refresh the token if needed)
		Token() (string, error)
	}
)

func (c *Client) Users(ctx context.Context) (user User, err error) {
	const resourceURL = "/users"
	data, err := c.request(ctx, resourceURL, nil)
	if err != nil {
		return User{}, err
	}
	var u User
	err = json.Unmarshal(data, &u)
	if err != nil {
		return User{}, errors.E(ErrUnexpectedResponseBody, err)
	}
	return u, nil
}

// MemberOrganizations returns all organizations which are associated with the user.
func (c *Client) MemberOrganizations(ctx context.Context) (orgs MemberOrganizations, err error) {
	const resourceURL = "/organizations"

	data, err := c.request(ctx, resourceURL, nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &orgs)
	if err != nil {
		return nil, errors.E(err, ErrUnexpectedResponseBody)
	}

	err = orgs.Validate()
	if err != nil {
		return nil, errors.E(ErrUnexpectedResponseBody, err)
	}
	return orgs, nil
}

func (c *Client) request(ctx context.Context, resourceURL string, postBody io.Reader) (data []byte, err error) {
	if c.Credential == nil {
		return nil, errors.E("no credential provided to %s endpoint", c.endpoint(resourceURL))
	}

	req, err := c.newRequest(ctx, "GET", resourceURL, postBody)
	if err != nil {
		return nil, err
	}

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.E(ErrNotFound)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.E(ErrUnexpectedStatus, "status: %s", resp.Status)
	}

	if ctype := resp.Header.Get("Content-Type"); ctype != contentType {
		return nil, errors.E(ErrUnexpectedResponseBody, "client expects the Content-Type: %s but got %s", contentType, ctype)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) newRequest(ctx context.Context, method string, relativeURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(relativeURL), body)
	if err != nil {
		return nil, err
	}
	token, err := c.Credential.Token()
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Context-Type", contentType)
	return req, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	return c.HTTPClient
}

func (c *Client) endpoint(url string) string {
	if c.BaseURL == "" {
		c.BaseURL = BaseURL
	}
	return fmt.Sprintf("%s%s", c.BaseURL, url)
}

const contentType = "application/json"
