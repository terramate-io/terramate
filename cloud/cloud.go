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
	"net/url"
	"os"
	"path"
	"strings"

	hversion "github.com/apparentlymart/go-versions/versions"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
	"github.com/terramate-io/terramate/versions"
)

// Host of the official Terramate Cloud API.
const Host = "api.terramate.io"

// BaseURL is the default cloud.terramate.io base API URL.
const BaseURL = "https://" + Host

const defaultPageSize = 50

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
	// ReviewRequestsPath is the review requests endpoint base path.
	ReviewRequestsPath = "/v1/review_requests"
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

var (
	pageSize         int64 = defaultPageSize
	debugAPIRequests bool
)

func init() {
	if d := os.Getenv("TMC_API_DEBUG"); d == "1" || d == "true" {
		debugAPIRequests = true
	}
	if sizeStr := os.Getenv("TMC_API_PAGESIZE"); sizeStr != "" {
		size, _ := strconv.Atoi64(sizeStr)
		if size != 0 {
			pageSize = int64(size)
		}
	}
}

// CheckVersion checks if current Terramate version can be used to communicate
// with the cloud.
func (c *Client) CheckVersion(ctx context.Context) error {
	client := &Client{
		BaseURL: c.BaseURL,
		noauth:  true,
	}
	wk, err := Get[WellKnown](ctx, client, client.URL(WellKnownCLIPath))
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
	return Get[User](ctx, c, c.URL(UsersPath))
}

// MemberOrganizations returns all organizations which are associated with the user.
func (c *Client) MemberOrganizations(ctx context.Context) (orgs MemberOrganizations, err error) {
	return Get[MemberOrganizations](ctx, c, c.URL(MembershipsPath))
}

// StacksByStatus returns all stacks for the given organization.
// It paginates as needed and returns the total stacks response.
func (c *Client) StacksByStatus(ctx context.Context, orgUUID UUID, repository string, target string, stackFilters StatusFilters) ([]StackObject, error) {
	path := path.Join(StacksPath, string(orgUUID))
	query := url.Values{}
	query.Set("repository", repository)
	if target != "" {
		query.Set("target", target)
	}
	if stackFilters.StackStatus != stack.NoFilter {
		query.Set("status", stackFilters.StackStatus.String())
	}
	if stackFilters.DeploymentStatus != deployment.NoFilter {
		query.Set("deployment_status", stackFilters.DeploymentStatus.String())
	}
	if stackFilters.DriftStatus != drift.NoFilter {
		query.Set("drift_status", stackFilters.DriftStatus.String())
	}
	query.Set("per_page", strconv.Itoa64(pageSize))
	url := c.URL(path)
	lastPage := int64(1)
	var stacks []StackObject
	for {
		query.Set("page", strconv.Itoa64(lastPage))
		url.RawQuery = query.Encode()
		resp, err := Get[StacksResponse](ctx, c, url)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, resp.Stacks...)
		if int64(len(resp.Stacks)) < pageSize {
			break
		}
		lastPage++
	}
	return stacks, nil
}

// ListReviewRequests retrieves the review requests for the given organization.
func (c *Client) ListReviewRequests(ctx context.Context, orgUUID UUID) (ReviewRequestResponses, error) {
	path := path.Join(ReviewRequestsPath, string(orgUUID))
	query := url.Values{}
	query.Set("per_page", strconv.Itoa64(pageSize))
	url := c.URL(path)
	lastPage := int64(1)
	var reviews ReviewRequestResponses
	for {
		query.Set("page", strconv.Itoa64(lastPage))
		url.RawQuery = query.Encode()
		resp, err := Get[ReviewRequestResponsePayload](ctx, c, url)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, resp.ReviewRequests...)
		if int64(len(resp.ReviewRequests)) < pageSize {
			break
		}
		lastPage++
	}
	return reviews, nil
}

// GetStack retrieves the details of the stack with given repo and metaID.
func (c *Client) GetStack(ctx context.Context, orgUUID UUID, repo, target, metaID string) (StackObject, bool, error) {
	query := url.Values{
		"repository": []string{repo},
		"target":     []string{target},
		"meta_id":    []string{strings.ToLower(metaID)},
	}

	url := c.URL(path.Join(StacksPath, string(orgUUID)), query)
	stacks, err := Get[StacksResponse](ctx, c, url)
	if err != nil {
		return StackObject{}, false, err
	}
	if len(stacks.Stacks) == 0 {
		return StackObject{}, false, nil
	}
	if len(stacks.Stacks) != 1 {
		return StackObject{}, false, errors.E("org+repo+target+meta_id must be unique. Unexpected TMC backend response")
	}
	return stacks.Stacks[0], true, nil
}

// StackLastDrift returns the drifts of the given stack.
func (c *Client) StackLastDrift(ctx context.Context, orgUUID UUID, stackID int64) (DriftsStackPayloadResponse, error) {
	path := path.Join(StacksPath, string(orgUUID), strconv.Itoa64(stackID), "drifts")
	query := url.Values{
		"page":     []string{"1"},
		"per_page": []string{"1"},
	}
	return Get[DriftsStackPayloadResponse](ctx, c, c.URL(path, query))
}

// DriftDetails retrieves details of the given driftID.
func (c *Client) DriftDetails(ctx context.Context, orgUUID UUID, stackID int64, driftID int64) (Drift, error) {
	path := path.Join(DriftsPath, string(orgUUID), strconv.Itoa64(stackID), strconv.Itoa64(driftID))
	return Get[Drift](ctx, c, c.URL(path))
}

// CreateDeploymentStacks creates a new deployment for provided stacks payload.
func (c *Client) CreateDeploymentStacks(
	ctx context.Context,
	orgUUID UUID,
	deploymentUUID UUID,
	deploymentStacksPayload DeploymentStacksPayloadRequest,
) (DeploymentStacksResponse, error) {
	if deploymentUUID == "" {
		panic(errors.E(errors.ErrInternal, "deploymentUUID must not be empty"))
	}
	err := deploymentStacksPayload.Validate()
	if err != nil {
		return DeploymentStacksResponse{}, errors.E(err, "failed to prepare the request")
	}
	return Post[DeploymentStacksResponse](
		ctx,
		c,
		deploymentStacksPayload,
		c.URL(path.Join(DeploymentsPath, string(orgUUID), string(deploymentUUID), "stacks")),
	)
}

// UpdateDeploymentStacks updates the deployment status of each stack in the payload set.
func (c *Client) UpdateDeploymentStacks(ctx context.Context, orgUUID UUID, deploymentUUID UUID, payload UpdateDeploymentStacks) error {
	_, err := Patch[EmptyResponse](
		ctx,
		c,
		payload,
		c.URL(path.Join(DeploymentsPath, string(orgUUID), string(deploymentUUID), "stacks")),
	)
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
		c.URL(path.Join(DriftsPath, string(orgUUID))),
	)
}

// SyncCommandLogs sends a batch of command logs to Terramate Cloud.
func (c *Client) SyncCommandLogs(
	ctx context.Context,
	orgUUID UUID,
	stackID int64,
	deploymentUUID UUID,
	logs CommandLogs,
	stackPreviewID string,
) error {
	err := logs.Validate()
	if err != nil {
		return errors.E(err, "failed to prepare the request")
	}

	url := c.URL(path.Join(
		StacksPath, string(orgUUID), strconv.Itoa64(stackID), "deployments", string(deploymentUUID), "logs",
	))

	// if the command logs are for a stack preview, use the stack preview url.
	if stackPreviewID != "" {
		url = c.URL(path.Join(StackPreviewsPath, string(orgUUID), stackPreviewID, "logs"))
	}

	_, err = Post[EmptyResponse](ctx, c, logs, url)
	return err
}

// Get requests the endpoint components list making a GET request and decode the response into the
// entity T if validates successfully.
func Get[T Resource](ctx context.Context, client *Client, u url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "GET", u, nil)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Post requests the endpoint components list making a POST request and decode the response into the
// entity T if validates successfully.
func Post[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "POST", url, bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Patch requests the endpoint components list making a PATCH request and decode the response into the
// entity T if validates successfully.
func Patch[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "PATCH", url, bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Put requests the endpoint components list making a PUT request and decode the
// response into the entity T if validated successfully.
func Put[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error) {
	dataPayload, err := json.Marshal(payload)
	if err != nil {
		return entity, errors.E(err, "marshaling request payload")
	}
	resource, err := Request[T](ctx, client, "PUT", url, bytes.NewBuffer(dataPayload))
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Request makes a request to the Terramate Cloud using client.
// The instantiated type gets decoded and return as the entity T,
func Request[T Resource](ctx context.Context, c *Client, method string, url url.URL, postBody io.Reader) (entity T, err error) {
	if !c.noauth && c.Credential == nil {
		return entity, errors.E("no credential provided to %s endpoint", url)
	}

	req, err := c.newRequest(ctx, method, url, postBody)
	if err != nil {
		return entity, err
	}

	if debugAPIRequests {
		data, _ := dumpRequest(req)
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
		return entity, errors.E(ErrNotFound, "%s %s", method, url.String())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return entity, errors.E(ErrUnexpectedStatus, "%s: status: %s, content: %s", url.String(), resp.Status, data)
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

func (c *Client) newRequest(ctx context.Context, method string, url url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Set("Content-Type", contentType)

	if !c.noauth {
		token, err := c.Credential.Token()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	return c.HTTPClient
}

// URL builds an URL for the given path and queries from the client's base URL.
func (c *Client) URL(path string, queries ...url.Values) url.URL {
	if c.BaseURL == "" {
		c.BaseURL = BaseURL
	}
	// c.BaseURL must be a valid URL.
	u, _ := url.Parse(c.BaseURL)
	u.Path = path

	query := url.Values{}
	for _, q := range queries {
		for k, v := range q {
			query[k] = v
		}
	}
	u.RawQuery = query.Encode()
	return *u
}

// dumpRequest returns a string representation of the request with the
// Authorization header redacted.
func dumpRequest(req *http.Request) ([]byte, error) {
	reqCopy := req.Clone(req.Context())

	var err error
	if req.GetBody != nil {
		reqCopy.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}

	reqCopy.Header.Set("Authorization", "REDACTED")

	return httputil.DumpRequestOut(reqCopy, true)
}

const contentType = "application/json"
