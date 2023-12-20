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
	"mime"
	"net/http"
	"net/http/httputil"
	"os"
	"path"

	hversion "github.com/apparentlymart/go-versions/versions"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
	"github.com/terramate-io/terramate/versions"
)

// Host of the official Terramate Cloud API.
const Host = "api.terramate.io"

// BaseURL is the default cloud.terramate.io base API URL.
const BaseURL = "https://" + Host

const (
	// WellKnownCLIPath is the well-known base path.
	WellKnownCLIPath = "/.well-known/cli.json"

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

		Logger *zerolog.Logger
		noauth bool
	}

	// Credential is the interface for the credential providers.
	Credential interface {
		// Token retrieves a new token ready be used (the credential provider must refresh the token if needed)
		Token() (string, error)
	}
)

var debugAPIRequests bool

func init() {
	if d := os.Getenv("TMC_API_DEBUG"); d == "1" || d == "true" {
		debugAPIRequests = true
	}
}

// CheckVersion checks if current Terramate version can be used to communicate
// with the cloud.
func (c *Client) CheckVersion(ctx context.Context) error {
	client := &Client{
		BaseURL: c.BaseURL,
		noauth:  true,
	}
	wk, err := Get[WellKnown](ctx, client, WellKnownCLIPath)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Trace().Err(err).Msgf("retrieving %s", WellKnownCLIPath)
		}
		return nil
	}
	version := hversion.MustParseVersion(terramate.Version())
	version.Prerelease = ""
	return versions.Check(version.String(), wk.RequiredVersion, false)
}

// Users retrieves the user details for the signed in user.
func (c *Client) Users(ctx context.Context) (user User, err error) {
	return Get[User](ctx, c, UsersPath)
}

// MemberOrganizations returns all organizations which are associated with the user.
func (c *Client) MemberOrganizations(ctx context.Context) (orgs MemberOrganizations, err error) {
	return Get[MemberOrganizations](ctx, c, MembershipsPath)
}

// StacksByStatus returns all stacks for the given organization.
func (c *Client) StacksByStatus(ctx context.Context, orgUUID UUID, status stack.FilterStatus) (StacksResponse, error) {
	path := path.Join(StacksPath, string(orgUUID))
	if status != stack.NoFilter {
		path += "?status=" + status.String()
	}
	return Get[StacksResponse](ctx, c, path)
}

// GetStack retrieves the details of the stack with given repo and metaID.
func (c *Client) GetStack(ctx context.Context, orgUUID UUID, repo, metaID string) (StackResponse, bool, error) {
	path := path.Join(StacksPath, string(orgUUID))
	path += fmt.Sprintf("?repository=%s&meta_id=%s", repo, metaID)
	stacks, err := Get[StacksResponse](ctx, c, path)
	if err != nil {
		return StackResponse{}, false, err
	}
	if len(stacks.Stacks) == 0 {
		return StackResponse{}, false, nil
	}
	if len(stacks.Stacks) != 1 {
		return StackResponse{}, false, errors.E("org+repo+meta_id must be unique. Unexpected TMC backend response")
	}
	return stacks.Stacks[0], true, nil
}

// StackDrifts returns the drifts of the given stack.
func (c *Client) StackDrifts(ctx context.Context, orgUUID UUID, stackID int64, page, perPage int) (DriftsStackPayloadResponse, error) {
	path := path.Join(StacksPath, string(orgUUID), strconv.Itoa64(stackID), "drifts")
	path += fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	return Get[DriftsStackPayloadResponse](ctx, c, path)
}

// DriftDetails retrieves details of the given driftID.
func (c *Client) DriftDetails(ctx context.Context, orgUUID UUID, stackID, driftID int64) (Drift, error) {
	path := path.Join(DriftsPath, string(orgUUID), strconv.Itoa64(stackID), strconv.Itoa64(driftID))
	return Get[Drift](ctx, c, path)
}

// CreateDeploymentStacks creates a new deployment for provided stacks payload.
func (c *Client) CreateDeploymentStacks(
	ctx context.Context,
	orgUUID UUID,
	deploymentUUID UUID,
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
		string(orgUUID),
		string(deploymentUUID),
		"stacks",
	)
}

// UpdateDeploymentStacks updates the deployment status of each stack in the payload set.
func (c *Client) UpdateDeploymentStacks(ctx context.Context, orgUUID UUID, deploymentUUID UUID, payload UpdateDeploymentStacks) error {
	_, err := Patch[EmptyResponse](ctx, c, payload, DeploymentsPath, string(orgUUID), string(deploymentUUID), "stacks")
	return err
}

// CreateStackDrift pushes a new drift status for the given stack.
func (c *Client) CreateStackDrift(
	ctx context.Context,
	orgUUID UUID,
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
		string(orgUUID),
	)
}

// SyncDeploymentLogs sends a batch of deployment logs to Terramate Cloud.
func (c *Client) SyncDeploymentLogs(
	ctx context.Context,
	orgUUID UUID,
	stackID int64,
	deploymentUUID UUID,
	logs DeploymentLogs,
) error {
	err := logs.Validate()
	if err != nil {
		return errors.E(err, "failed to prepare the request")
	}
	// Endpoint:/v1/stacks/{org_uuid}/{stack_id}/deployments/{deployment_uuid}/logs
	_, err = Post[EmptyResponse](
		ctx, c, logs,
		StacksPath, string(orgUUID), strconv.Itoa64(stackID), "deployments", string(deploymentUUID), "logs",
	)
	return err
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
	if !c.noauth && c.Credential == nil {
		return entity, errors.E("no credential provided to %s endpoint", c.endpoint(resourceURL))
	}

	req, err := c.newRequest(ctx, method, resourceURL, postBody)
	if err != nil {
		return entity, err
	}

	if debugAPIRequests {
		data, _ := httputil.DumpRequestOut(req, true)
		fmt.Printf(">>> %s\n\n", data)
	}

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return entity, err
	}

	if debugAPIRequests {
		data, _ := httputil.DumpResponse(resp, true)
		fmt.Printf("<<< %s\n\n", data)
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

	if ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); ctype != contentType {
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

	req.Header.Add("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Add("Content-Type", contentType)

	if !c.noauth {
		token, err := c.Credential.Token()
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+token)
	}
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
