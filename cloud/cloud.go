// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package cloud implements a client SDK for communication with the cloud API.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
)

// Host of the official Terramate Cloud API.
const Host = "api.terramate.io"

// BaseURL is the default cloud.terramate.io base API URL.
const BaseURL = "https://" + Host

const (
	// UsersPath is the users endpoint base path.
	UsersPath = "/v1/users"
	// MembershipsPath is the memberships endpoint base path.
	MembershipsPath = "/v1/memberships"
	// DeploymentsPath is the deployments endpoint base path.
	DeploymentsPath = "/v1/deployments"
	// DriftsPath is the drifts endpoint base path.
	DriftsPath = "/v1/drifts"
	// StacksPath is the stacks endpoint base path.
	StacksPath = "/v1/stacks"
)

// ErrUnexpectedStatus indicates the server responded with an unexpected status code.
const ErrUnexpectedStatus errors.Kind = "unexpected status code"

// ErrNotFound indicates the requested resource does not exist in the server.
const ErrNotFound errors.Kind = "resource not found (HTTP Status 404)"

// ErrUnexpectedResponseBody indicates the server responded with an unexpected body.
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"

type (
	// Client is the cloud SDK client.
	Client struct {
		// BaseURL is the cloud base endpoint URL.
		// If not set, it defaults to [BaseURL].
		BaseURL    string
		IDPKey     string
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

// Users retrieves the user details for the signed in user.
func (c *Client) Users(ctx context.Context) (user User, err error) {
	return Get[User](ctx, c, UsersPath)
}

// MemberOrganizations returns all organizations which are associated with the user.
func (c *Client) MemberOrganizations(ctx context.Context) (orgs MemberOrganizations, err error) {
	return Get[MemberOrganizations](ctx, c, MembershipsPath)
}

// Stacks returns all stacks for the given organization.
func (c *Client) Stacks(ctx context.Context, orgUUID string, status stack.FilterStatus) (StacksResponse, error) {
	path := path.Join(StacksPath, orgUUID)
	if status == stack.UnhealthyFilter {
		path += "?status=" + status.String()
	}
	return Get[StacksResponse](ctx, c, path)
}

// CreateDeploymentStacks creates a new deployment for provided stacks payload.
func (c *Client) CreateDeploymentStacks(
	ctx context.Context,
	orgUUID string,
	deploymentUUID string,
	deploymentStacksPayload DeploymentStacksPayloadRequest,
) (DeploymentStacksResponse, error) {
	err := deploymentStacksPayload.Validate()
	if err != nil {
		return DeploymentStacksResponse{}, errors.E(err, "failed to prepare the request")
	}
	return Post[DeploymentStacksResponse](
		ctx,
		c,
		deploymentStacksPayload,
		DeploymentsPath,
		orgUUID,
		deploymentUUID,
		"stacks",
	)
}

// UpdateDeploymentStacks updates the deployment status of each stack in the payload set.
func (c *Client) UpdateDeploymentStacks(ctx context.Context, orgUUID string, deploymentUUID string, payload UpdateDeploymentStacks) error {
	_, err := Patch[EmptyResponse](ctx, c, payload, DeploymentsPath, orgUUID, deploymentUUID, "stacks")
	return err
}

// CreateStackDrift pushes a new drift status for the given stack.
func (c *Client) CreateStackDrift(
	ctx context.Context,
	orgUUID string,
	driftPayload DriftStackPayloadRequest,
) (EmptyResponse, error) {
	err := driftPayload.Validate()
	if err != nil {
		return EmptyResponse(""), errors.E(err, "failed to prepare the request")
	}
	return Post[EmptyResponse](
		ctx,
		c,
		driftPayload,
		DriftsPath,
		orgUUID,
	)
}

// Get requests the endpoint components list making a GET request and decode the response into the
// entity T if validates successfully.
func Get[T Resource](ctx context.Context, client *Client, endpoint ...string) (entity T, err error) {
	resource, err := Request[T](ctx, client, "GET", path.Join(endpoint...), nil)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Post requests the endpoint components list making a POST request and decode the response into the
// entity T if validates successfully.
func Post[T Resource](ctx context.Context, client *Client, payload interface{}, endpoint ...string) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "POST", path.Join(endpoint...), bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Patch requests the endpoint components list making a PATCH request and decode the response into the
// entity T if validates successfully.
func Patch[T Resource](ctx context.Context, client *Client, payload interface{}, endpoint ...string) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "PATCH", path.Join(endpoint...), bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Put requests the endpoint components list making a PUT request and decode the
// response into the entity T if validated successfully.
func Put[T Resource](ctx context.Context, client *Client, payload interface{}, endpoint ...string) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "PUT", path.Join(endpoint...), bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Request makes a request to the Terramate Cloud using client.
// The instantiated type gets decoded and return as the entity T,
func Request[T Resource](ctx context.Context, c *Client, method string, resourceURL string, postBody io.Reader) (entity T, err error) {
	if c.Credential == nil {
		return entity, errors.E("no credential provided to %s endpoint", c.endpoint(resourceURL))
	}

	req, err := c.newRequest(ctx, method, resourceURL, postBody)
	if err != nil {
		return entity, err
	}

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return entity, err
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return entity, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return entity, errors.E(ErrNotFound, "%s %s", method, c.endpoint(resourceURL))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return entity, errors.E(ErrUnexpectedStatus, "%s: status: %s, content: %s", c.endpoint(resourceURL), resp.Status, data)
	}

	if resp.StatusCode == http.StatusNoContent {
		return entity, nil
	}

	if ctype := resp.Header.Get("Content-Type"); ctype != contentType {
		return entity, errors.E(ErrUnexpectedResponseBody, "client expects the Content-Type: %s but got %s", contentType, ctype)
	}

	var resource T
	err = json.Unmarshal(data, &resource)
	if err != nil {
		return entity, errors.E(ErrUnexpectedResponseBody, err, "status: %d, data: %s", resp.StatusCode, data)
	}
	err = resource.Validate()
	if err != nil {
		return entity, errors.E(ErrUnexpectedResponseBody, err)
	}
	return resource, nil
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
	req.Header.Add("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", contentType)
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
