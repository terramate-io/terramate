// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"context"
	stdhttp "net/http"
	"net/url"
	"path"
	"strings"

	hversion "github.com/apparentlymart/go-versions/versions"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/http"
	"github.com/terramate-io/terramate/strconv"
	"github.com/terramate-io/terramate/versions"
)

// Client is the cloud SDK client.
type Client struct {
	// region where the client must connect. Default is EU.
	// Note: this is only used if the BaseURL is not set.
	region Region

	// baseURL is the cloud base endpoint URL.
	// If not set, it defaults to calling `BaseURL(client.Region)`.
	baseURL    string
	credential http.Credential

	// httpClient is the HTTP client reused in all connections.
	// if not set, a new instance of http.Client is created on the first request.
	httpClient *stdhttp.Client

	logger *zerolog.Logger
}

// Option is a functional option for the client.
type Option func(*Client)

// Options is a list of functional options.
type Options []Option

// NewClient creates a new cloud client. It uses the default region (EU) and
// calls BaseURL(region) to set the base URL if [WithBaseURL] is not provided.
func NewClient(opts ...Option) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	if c.baseURL == "" {
		c.baseURL = BaseURL(c.region)
	}
	return c
}

// WithBaseURL sets the base URL to be used in the client.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithRegion sets the region to be used in the client.
func WithRegion(region Region) Option {
	return func(c *Client) {
		c.region = region
	}
}

// WithLogger sets the logger to be used in the client.
func WithLogger(logger *zerolog.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithCredential sets the credential to be used in the client.
func WithCredential(credential http.Credential) Option {
	return func(c *Client) {
		c.credential = credential
	}
}

// WithHTTPClient sets the HTTP client to be used in the client.
func WithHTTPClient(httpClient *stdhttp.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// SetCredential sets the client cloud credential.
func (c *Client) SetCredential(credential http.Credential) {
	c.credential = credential
}

// Credential returns the cloud credential.
func (c *Client) Credential() http.Credential {
	return c.credential
}

// HTTPClient returns the HTTP client used by the client.
func (c *Client) HTTPClient() *stdhttp.Client {
	if c.httpClient == nil {
		c.httpClient = &stdhttp.Client{}
	}
	return c.httpClient
}

// BaseURL returns the API base URL of the client.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Region returns the region of the client.
func (c *Client) Region() Region {
	return c.region
}

// CheckVersion checks if current Terramate version can be used to communicate
// with the cloud.
func (c *Client) CheckVersion(ctx context.Context) error {
	client := NewClient(
		WithBaseURL(c.baseURL),
		WithRegion(c.region),
		WithHTTPClient(c.httpClient),
	)

	wk, err := http.Get[resources.WellKnown](ctx, client, client.URL(WellKnownCLIPath))
	if err != nil {
		if c.logger != nil {
			c.logger.Trace().Err(err).Msgf("retrieving %s", WellKnownCLIPath)
		}
		return nil
	}
	version := hversion.MustParseVersion(terramate.Version())
	version.Prerelease = ""
	return versions.Check(version.String(), wk.RequiredVersion, false)
}

// GetOrgSingleSignOnID returns the organization SSO ID.
func (c *Client) GetOrgSingleSignOnID(ctx context.Context, orgName string) (string, error) {
	client := NewClient(
		WithBaseURL(c.baseURL),
		WithRegion(c.region),
	)

	endpoint := path.Join(SingleSignOnDetailByNamePath, orgName)
	ssoDetails, err := http.Get[resources.SingleSignOnDetailResponse](ctx, client, client.URL(endpoint))
	if err != nil {
		return "", err
	}
	return ssoDetails.EnterpriseOrgID, nil
}

// Users retrieves the user details for the signed in user.
func (c *Client) Users(ctx context.Context) (user resources.User, err error) {
	return http.Get[resources.User](ctx, c, c.URL(UsersPath))
}

// MemberOrganizations returns all organizations which are associated with the user.
func (c *Client) MemberOrganizations(ctx context.Context) (orgs resources.MemberOrganizations, err error) {
	return http.Get[resources.MemberOrganizations](ctx, c, c.URL(MembershipsPath))
}

// StacksByStatus returns all stacks for the given organization.
// It paginates as needed and returns the total stacks response.
func (c *Client) StacksByStatus(ctx context.Context, orgUUID resources.UUID, repository string, target string, stackFilters resources.StatusFilters) ([]resources.StackObject, error) {
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
	var stacks []resources.StackObject
	for {
		query.Set("page", strconv.Itoa64(lastPage))
		url.RawQuery = query.Encode()
		resp, err := http.Get[resources.StacksResponse](ctx, c, url)
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
func (c *Client) ListReviewRequests(ctx context.Context, orgUUID resources.UUID) (resources.ReviewRequestResponses, error) {
	path := path.Join(ReviewRequestsPath, string(orgUUID))
	query := url.Values{}
	query.Set("per_page", strconv.Itoa64(pageSize))
	url := c.URL(path)
	lastPage := int64(1)
	var reviews resources.ReviewRequestResponses
	for {
		query.Set("page", strconv.Itoa64(lastPage))
		url.RawQuery = query.Encode()
		resp, err := http.Get[resources.ReviewRequestResponsePayload](ctx, c, url)
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
func (c *Client) GetStack(ctx context.Context, orgUUID resources.UUID, repo, target, metaID string) (resources.StackObject, bool, error) {
	query := url.Values{
		"repository": []string{repo},
		"target":     []string{target},
		"meta_id":    []string{strings.ToLower(metaID)},
	}

	url := c.URL(path.Join(StacksPath, string(orgUUID)), query)
	stacks, err := http.Get[resources.StacksResponse](ctx, c, url)
	if err != nil {
		return resources.StackObject{}, false, err
	}
	if len(stacks.Stacks) == 0 {
		return resources.StackObject{}, false, nil
	}
	if len(stacks.Stacks) != 1 {
		return resources.StackObject{}, false, errors.E("org+repo+target+meta_id must be unique. Unexpected TMC backend response")
	}
	return stacks.Stacks[0], true, nil
}

// StackLastDrift returns the drifts of the given stack.
func (c *Client) StackLastDrift(ctx context.Context, orgUUID resources.UUID, stackID int64) (resources.DriftsStackPayloadResponse, error) {
	path := path.Join(StacksPath, string(orgUUID), strconv.Itoa64(stackID), "drifts")
	query := url.Values{
		"page":     []string{"1"},
		"per_page": []string{"1"},
	}
	return http.Get[resources.DriftsStackPayloadResponse](ctx, c, c.URL(path, query))
}

// DriftDetails retrieves details of the given driftID.
func (c *Client) DriftDetails(ctx context.Context, orgUUID resources.UUID, stackID int64, driftID int64) (resources.Drift, error) {
	path := path.Join(DriftsPath, string(orgUUID), strconv.Itoa64(stackID), strconv.Itoa64(driftID))
	return http.Get[resources.Drift](ctx, c, c.URL(path))
}

// CreateDeploymentStacks creates a new deployment for provided stacks payload.
func (c *Client) CreateDeploymentStacks(
	ctx context.Context,
	orgUUID resources.UUID,
	deploymentUUID resources.UUID,
	deploymentStacksPayload resources.DeploymentStacksPayloadRequest,
) (resources.DeploymentStacksResponse, error) {
	if deploymentUUID == "" {
		panic(errors.E(errors.ErrInternal, "deploymentUUID must not be empty"))
	}
	err := deploymentStacksPayload.Validate()
	if err != nil {
		return resources.DeploymentStacksResponse{}, errors.E(err, "failed to prepare the request")
	}
	return http.Post[resources.DeploymentStacksResponse](
		ctx,
		c,
		deploymentStacksPayload,
		c.URL(path.Join(DeploymentsPath, string(orgUUID), string(deploymentUUID), "stacks")),
	)
}

// UpdateDeploymentStacks updates the deployment status of each stack in the payload set.
func (c *Client) UpdateDeploymentStacks(ctx context.Context, orgUUID resources.UUID, deploymentUUID resources.UUID, payload resources.UpdateDeploymentStacks) error {
	_, err := http.Patch[resources.EmptyResponse](
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
	orgUUID resources.UUID,
	driftPayload resources.DriftCheckRunStartPayloadRequest,
) (resources.DriftCheckRunStartResponse, error) {
	err := driftPayload.Validate()
	if err != nil {
		return resources.DriftCheckRunStartResponse{}, errors.E(err, "failed to prepare the request")
	}
	return http.Post[resources.DriftCheckRunStartResponse](
		ctx,
		c,
		driftPayload,
		c.URL(path.Join(DriftsPath, string(orgUUID))),
	)
}

// UpdateStackDrift updates the drift status for the given drift UUID using v2 API.
func (c *Client) UpdateStackDrift(
	ctx context.Context,
	orgUUID resources.UUID,
	driftUUID resources.UUID,
	driftPayload resources.UpdateDriftPayloadRequest,
) error {
	err := driftPayload.Validate()
	if err != nil {
		return errors.E(err, "failed to prepare the request")
	}
	_, err = http.Patch[resources.EmptyResponse](
		ctx,
		c,
		driftPayload,
		c.URL(path.Join(DriftsPath, string(orgUUID), string(driftUUID))),
	)
	return err
}

// SyncCommandLogs sends a batch of command logs to Terramate Cloud.
func (c *Client) SyncCommandLogs(
	ctx context.Context,
	orgUUID resources.UUID,
	stackID int64,
	deploymentUUID resources.UUID,
	logs resources.CommandLogs,
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

	_, err = http.Post[resources.EmptyResponse](ctx, c, logs, url)
	return err
}

// CreateStoreOutput creates a new output in the Terramate Cloud store.
func (c *Client) CreateStoreOutput(ctx context.Context, orgUUID resources.UUID, output resources.StoreOutputRequest) (resources.StoreOutput, error) {
	err := output.Validate()
	if err != nil {
		return resources.StoreOutput{}, errors.E(err, "failed to prepare the request")
	}
	return http.Post[resources.StoreOutput](
		ctx,
		c,
		output,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs")),
	)
}

// GetStoreOutput retrieves the output from the Terramate Cloud store.
func (c *Client) GetStoreOutput(ctx context.Context, orgUUID resources.UUID, id resources.UUID) (resources.StoreOutput, error) {
	return http.Get[resources.StoreOutput](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id))),
	)
}

// LookupStoreOutput retrieves the output from the Terramate Cloud store by its key.
func (c *Client) LookupStoreOutput(ctx context.Context, orgUUID resources.UUID, key resources.StoreOutputKey) (resources.StoreOutput, error) {
	query := url.Values{
		"repository":    []string{key.Repository},
		"stack_meta_id": []string{key.StackMetaID},
		"name":          []string{string(key.Name)},
	}
	if key.Target != "" {
		// let backend choose the default value.
		query.Set("target", string(key.Target))
	}
	return http.Get[resources.StoreOutput](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs"), query),
	)
}

// UpdateStoreOutputValue updates the value of the output in the Terramate Cloud store.
func (c *Client) UpdateStoreOutputValue(ctx context.Context, orgUUID resources.UUID, id resources.UUID, value string) error {
	_, err := http.Put[resources.EmptyResponse](
		ctx,
		c,
		value,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id), "value")),
	)
	return err
}

// DeleteStoreOutput deletes the output from the Terramate Cloud store.
func (c *Client) DeleteStoreOutput(ctx context.Context, orgUUID resources.UUID, id resources.UUID) error {
	return http.Delete[resources.EmptyResponse](
		ctx,
		c,
		c.URL(path.Join(StorePath, string(orgUUID), "outputs", string(id))),
	)
}

// URL builds an URL for the given path and queries from the client's base URL.
func (c *Client) URL(path string, queries ...url.Values) url.URL {
	// c.BaseURL must be a valid URL.
	u, _ := url.Parse(c.baseURL)
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
