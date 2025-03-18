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
	"sync"
	"time"

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

// BaseDomain is the Terramate Cloud base domain.
const apiBaseDomain = "api.terramate.io"

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
	// StorePath is the store endpoint base path.
	StorePath = "/v1/store"
)

// DefaultTimeout is a (optional) good default timeout to be used by TMC clients.
const DefaultTimeout = 60 * time.Second

// ErrUnexpectedStatus indicates the server responded with an unexpected status code.
const ErrUnexpectedStatus errors.Kind = "unexpected status code"

// ErrNotFound indicates the requested resource does not exist in the server.
const ErrNotFound errors.Kind = "resource not found (HTTP Status 404)"

// ErrConflict indicates the request contains conflicting data (eg.: duplicated resource)
const ErrConflict errors.Kind = "conflict (HTTP Status 409)"

// ErrUnexpectedResponseBody indicates the server responded with an unexpected body.
const ErrUnexpectedResponseBody errors.Kind = "unexpected API response body"

type (
	// Client is the cloud SDK client.
	Client struct {
		// Region where the client must connect. Default is EU.
		// Note: this is only used if the BaseURL is not set.
		Region Region

		// BaseURL is the cloud base endpoint URL.
		// If not set, it defaults to calling `BaseURL(client.Region)`.
		BaseURL    string
		Credential Credential

		// HTTPClient is the HTTP client reused in all connections.
		// if not set, a new instance of http.Client is created on the first request.
		HTTPClient *http.Client

		Logger *zerolog.Logger
		noauth bool

		// mu guards the changes in Client attributes concurrently.
		mu sync.Mutex
	}

	// Region is the Terramate Cloud region (EU, US, etc).
	Region int

	// Regions is a list of cloud regions.
	Regions []Region

	// Credential is the interface for the credential providers.
	Credential interface {
		// ApplyCredentials applies the credential to the given request.
		ApplyCredentials(req *http.Request) error

		// RedactCredentials redacts the credential from the given request.
		// This is used for dumping the request without exposing the credential.
		RedactCredentials(req *http.Request)
	}
)

// Available cloud locations.
const (
	// For backward compatibility we want the zero value to be the default
	// if not set in the [cloud.Client] struct.
	EU Region = iota
	US
	invalidRegion
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

// BaseURL returns the API base URL for the Terramate Cloud.
func BaseURL(region Region) string {
	if region == EU {
		return "https://" + apiBaseDomain
	}
	return "https://" + region.String() + "." + apiBaseDomain
}

// BaseDomain returns the API base domain for the Terramate Cloud.
func BaseDomain(region Region) string {
	if region == EU {
		return apiBaseDomain
	}
	return region.String() + "." + apiBaseDomain
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

// CreateStoreOutput creates a new output in the Terramate Cloud store.
func (c *Client) CreateStoreOutput(ctx context.Context, orgUUID UUID, output StoreOutputRequest) (StoreOutput, error) {
	err := output.Validate()
	if err != nil {
		return StoreOutput{}, errors.E(err, "failed to prepare the request")
	}
	return Post[StoreOutput](
		ctx,
		c,
		output,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs")),
	)
}

// GetStoreOutput retrieves the output from the Terramate Cloud store.
func (c *Client) GetStoreOutput(ctx context.Context, orgUUID UUID, id UUID) (StoreOutput, error) {
	return Get[StoreOutput](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id))),
	)
}

// LookupStoreOutput retrieves the output from the Terramate Cloud store by its key.
func (c *Client) LookupStoreOutput(ctx context.Context, orgUUID UUID, key StoreOutputKey) (StoreOutput, error) {
	query := url.Values{
		"repository":    []string{key.Repository},
		"stack_meta_id": []string{key.StackMetaID},
		"name":          []string{string(key.Name)},
	}
	if key.Target != "" {
		// let backend choose the default value.
		query.Set("target", string(key.Target))
	}
	return Get[StoreOutput](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs"), query),
	)
}

// UpdateStoreOutputValue updates the value of the output in the Terramate Cloud store.
func (c *Client) UpdateStoreOutputValue(ctx context.Context, orgUUID UUID, id UUID, value string) error {
	_, err := Put[EmptyResponse](
		ctx,
		c,
		value,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id), "value")),
	)
	return err
}

// DeleteStoreOutput deletes the output from the Terramate Cloud store.
func (c *Client) DeleteStoreOutput(ctx context.Context, orgUUID UUID, id UUID) error {
	return Delete[EmptyResponse](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id))),
	)
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
func Post[T Resource](ctx context.Context, client *Client, payload any, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "POST", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Patch requests the endpoint components list making a PATCH request and decode the response into the
// entity T if validates successfully.
func Patch[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "PATCH", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Put requests the endpoint components list making a PUT request and decode the
// response into the entity T if validated successfully.
func Put[T Resource](ctx context.Context, client *Client, payload interface{}, url url.URL) (entity T, err error) {
	resource, err := Request[T](ctx, client, "PUT", url, payload)
	if err != nil {
		return entity, err
	}
	return resource, nil
}

// Delete requests the endpoint url with a DELETE method.
func Delete[T Resource](ctx context.Context, client *Client, url url.URL) error {
	_, err := Request[T](ctx, client, "DELETE", url, nil)
	return err
}

// Request makes a request to the Terramate Cloud using client.
// The instantiated type gets decoded and return as the entity T,
// The payload is encoded accordingly to the rules below:
// - If payload is nil, no body is sent and no Content-Type is set.
// - If payload is a []byte or string, it is sent as is and the Content-Type is set to text/plain.
// - If payload is any other type, it is marshaled to JSON and the Content-Type is set to application/json.
func Request[T Resource](ctx context.Context, c *Client, method string, url url.URL, payload any) (res T, err error) {
	if !c.noauth && c.Credential == nil {
		return res, errors.E("no credential provided to %s endpoint", url)
	}

	req, err := c.newRequest(ctx, method, url, payload)
	if err != nil {
		return res, err
	}

	if debugAPIRequests {
		data, _ := c.dumpRequest(req)
		fmt.Printf(">>> %s\n\n", data)
	}

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return res, err
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
		return res, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return res, errors.E(ErrNotFound, "%s %s", method, url.String())
	}

	if resp.StatusCode == http.StatusConflict {
		return res, errors.E(ErrConflict, "%s %s", method, url.String())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return res, errors.E(ErrUnexpectedStatus, "%s: status: %d, content: %s", url.String(), resp.StatusCode, data)
	}

	if resp.StatusCode == http.StatusNoContent {
		return res, nil
	}

	ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ctype != objectContentType {
		return res, errors.E(ErrUnexpectedResponseBody, "client expects the Content-Type: %s but got %s", objectContentType, ctype)
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return res, errors.E(ErrUnexpectedResponseBody, err, "status: %d, data: %s", resp.StatusCode, data)
	}
	err = res.Validate()
	if err != nil {
		return res, errors.E(ErrUnexpectedResponseBody, err)
	}
	return res, nil
}

func (c *Client) newRequest(ctx context.Context, method string, url url.URL, payload any) (*http.Request, error) {
	body, ctype, err := preparePayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "terramate/v"+terramate.Version())
	req.Header.Set("Content-Type", ctype)

	if !c.noauth {
		err := c.Credential.ApplyCredentials(req)
		if err != nil {
			return nil, err
		}
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
	c.mu.Lock()
	if c.BaseURL == "" {
		c.BaseURL = BaseURL(c.Region)
	}
	c.mu.Unlock()
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
// Authentication/Authorization header redacted.
func (c *Client) dumpRequest(req *http.Request) ([]byte, error) {
	reqCopy := req.Clone(req.Context())

	var err error
	if req.GetBody != nil {
		reqCopy.Body, err = req.GetBody()
		if err != nil {
			return nil, err
		}
	}

	if !c.noauth {
		c.Credential.RedactCredentials(reqCopy)
	}

	return httputil.DumpRequestOut(reqCopy, true)
}

// ParseRegion parses a user-supplied region name.
func ParseRegion(str string) (Region, error) {
	switch str {
	case "eu":
		return EU, nil
	case "us":
		return US, nil
	default:
		return invalidRegion, errors.E("unknown cloud region: %s", str)
	}
}

// String returns the string representation of the region.
func (r Region) String() string {
	switch r {
	case EU:
		return "eu"
	case US:
		return "us"
	default:
		panic(errors.E("invalid region", r))
	}
}

// String returns the string representation of the regions list.
func (rs Regions) String() string {
	var regions []string
	for _, r := range rs {
		regions = append(regions, r.String())
	}
	return strings.Join(regions, ", ")
}

// AvailableRegions returns a list of available cloud regions.
func AvailableRegions() Regions {
	return Regions{EU, US}
}

// HTMLURL returns the Terramate Cloud frontend URL.
func HTMLURL(region Region) string {
	if region == EU {
		return "https://cloud.terramate.io"
	}
	return "https://" + region.String() + ".cloud.terramate.io"
}

func preparePayload(payload any) (body io.Reader, ctype string, err error) {
	if payload != nil {
		switch v := payload.(type) {
		case []byte:
			body = bytes.NewBuffer(v)
			ctype = "text/plain"
		case string:
			body = strings.NewReader(v)
			ctype = "text/plain"
		default:
			data, err := json.Marshal(payload)
			if err != nil {
				return nil, "", errors.E("marshaling request payload", err)
			}
			body = bytes.NewBuffer(data)
			ctype = objectContentType
		}
	}
	return body, ctype, nil
}

// ParseStatusFilters parses the set of Terramate Cloud filters and return an error if any of them
// is not recognized. If any argument is an empty string then it returns its corresponding <type>.NoFilter.
func ParseStatusFilters(stackStatus, deploymentStatus, driftStatus string) (StatusFilters, error) {
	stackStatusFilter, err := parseStackStatusFilter(stackStatus)
	if err != nil {
		return NoStatusFilters(), err
	}
	deploymentStatusFilter, err := parseDeploymentStatusFilter(deploymentStatus)
	if err != nil {
		return NoStatusFilters(), err
	}
	driftStatusFilter, err := parseDriftStatusFilter(driftStatus)
	if err != nil {
		return NoStatusFilters(), err
	}
	return StatusFilters{
		StackStatus:      stackStatusFilter,
		DeploymentStatus: deploymentStatusFilter,
		DriftStatus:      driftStatusFilter,
	}, nil
}

func parseStackStatusFilter(filterStr string) (stack.FilterStatus, error) {
	if filterStr == "" {
		return stack.NoFilter, nil
	}
	filter, err := stack.NewStatusFilter(filterStr)
	if err != nil {
		return stack.NoFilter, errors.E(err, "unrecognized stack filter")
	}
	return filter, nil
}

func parseDeploymentStatusFilter(filterStr string) (deployment.FilterStatus, error) {
	if filterStr == "" {
		return deployment.NoFilter, nil
	}
	filter, err := deployment.NewStatusFilter(filterStr)
	if err != nil {
		return deployment.NoFilter, errors.E(err, "unrecognized deployment filter")
	}
	return filter, nil
}

func parseDriftStatusFilter(filterStr string) (drift.FilterStatus, error) {
	if filterStr == "" {
		return drift.NoFilter, nil
	}
	filter, err := drift.NewStatusFilter(filterStr)
	if err != nil {
		return drift.NoFilter, errors.E(err, "unrecognized drift filter")
	}
	return filter, nil
}

const objectContentType = "application/json"
