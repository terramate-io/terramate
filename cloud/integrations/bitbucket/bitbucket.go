// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/terramate-io/terramate/errors"
)

// DefaultTimeout is the default timeout for Bitbucket API requests.
const DefaultTimeout = 60 * time.Second

type (
	// Client is a Bitbucket Cloud client.
	Client struct {
		// BaseURL is the base URL used to construct the final URL of endpoints.
		// For Bitbucket Cloud, it should be https://api.bitbucket.org/2.0
		BaseURL string

		// HTTPClient is the HTTP client used to make requests.
		// if not set, a new http.Client is used on each request.
		HTTPClient *http.Client

		// Token is the Bitbucket Cloud token.
		Token string

		// Workspace is the Bitbucket Cloud workspace.
		Workspace string

		// RepoSlug is the Bitbucket Cloud repository slug.
		RepoSlug string
	}

	// PRs is a list of Bitbucket Pull Requests.
	PRs []PR

	// RenderedContent is the rendered version of the PR content.
	RenderedContent struct {
		Raw    string `json:"raw"`
		Markup string `json:"markup"`
		HTML   string `json:"html"`
	}

	// Rendered is the rendered version of the PR metadata.
	Rendered struct {
		Title       RenderedContent `json:"title"`
		Description RenderedContent `json:"description"`
		Reason      RenderedContent `json:"reason"`
	}

	// Summary is a Bitbucket Pull Request summary.
	Summary RenderedContent

	// Links is a collection of Bitbucket links.
	Links struct {
		Self *struct {
			Href string `json:"href,omitempty"`
		} `json:"self"`
		HTML *struct {
			Href string `json:"href,omitempty"`
		} `json:"html"`
		Avatar struct {
			Href string `json:"href,omitempty"`
		} `json:"avatar"`
	}

	// User is a Bitbucket user.
	User struct {
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
		Links       Links  `json:"links"`
		UUID        string `json:"uuid"`
		AccountID   string `json:"account_id"`
		Nickname    string `json:"nickname"`
	}

	// Actor is a Bitbucket actor.
	Actor struct {
		Type           string `json:"type"`
		User           User   `json:"user,omitempty"`
		Role           string `json:"role,omitempty"`
		Approved       *bool  `json:"approved,omitempty"`
		State          any    `json:"state,omitempty"`
		ParticipatedOn string `json:"participated_on,omitempty"`
	}

	// Commit is a Bitbucket commit.
	Commit struct {
		ShortHash string `json:"hash"`
		Date      string `json:"date"`

		// Note: this is not part of the Bitbucket API response.
		// This is fetched from the commit API and stored here for convenience.
		SHA string
	}

	// Branch is a Bitbucket branch.
	Branch struct {
		Name                 string   `json:"name"`
		MergeStrategies      []string `json:"merge_strategies"`
		DefaultMergeStrategy string   `json:"default_merge_strategy"`
	}

	// TargetBranch is the source or destination branch of a pull request.
	TargetBranch struct {
		Repository struct {
			Type string `json:"type"`
		}
		Branch Branch `json:"branch"`
		Commit Commit `json:"commit"`
	}

	// PR is a Bitbucket Pull Request.
	PR struct {
		Type              string       `json:"type"`
		ID                int          `json:"id"`
		Title             string       `json:"title"`
		Rendered          Rendered     `json:"rendered"`
		Summary           Summary      `json:"summary"`
		State             string       `json:"state"`
		Author            User         `json:"author"`
		Source            TargetBranch `json:"source"`
		Destination       TargetBranch `json:"destination"`
		MergeCommit       Commit       `json:"merge_commit"`
		CommentCount      int          `json:"comment_count"`
		TaskCount         int          `json:"task_count"`
		CloseSourceBranch bool         `json:"close_source_branch"`
		ClosedBy          *Actor       `json:"closed_by,omitempty"`
		Reason            string       `json:"reason"`
		CreatedOn         string       `json:"created_on"`
		UpdatedOn         string       `json:"updated_on"`
		Reviewers         []User       `json:"reviewers"`
		Participants      []Actor      `json:"participants"`
		Links             Links        `json:"links"`
	}

	// PullRequestResponse is the response of a pull request list.
	PullRequestResponse struct {
		Size     int    `json:"size"`
		Page     int    `json:"page"`
		PageLen  int    `json:"pagelen"`
		Next     string `json:"next"`
		Previous string `json:"previous"`
		Values   []PR   `json:"values"`
	}
)

// GetPullRequestsForCommit fetches a list of pull requests that contain the given commit.
// It filters by branch first (required) and then filters by commit hash client-side.
func (c *Client) GetPullRequestsForCommit(ctx context.Context, commit, branch string) (prs []PR, err error) {
	if branch == "" {
		// Fallback to legacy endpoint for tag builds or when branch is unknown.
		// This endpoint can be unreliable with pipeline tokens (requires specific app permissions),
		// but it's the only option when we don't have a branch to search by.
		// See: https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pullrequests/#api-repositories-workspace-repo-slug-commit-commit-pullrequests-get
		url := fmt.Sprintf("%s/repositories/%s/%s/commit/%s/pullrequests",
			c.baseURL(), c.Workspace, c.RepoSlug, commit)
		return c.getPullRequests(ctx, url, nil)
	}

	// Escape special characters in branch name for the query
	escapedBranch := strings.ReplaceAll(branch, "\\", "\\\\")
	escapedBranch = strings.ReplaceAll(escapedBranch, "\"", "\\\"")

	query := fmt.Sprintf("(source.branch.name=\"%s\" OR destination.branch.name=\"%s\") AND (state=\"OPEN\" OR state=\"MERGED\" OR state=\"DECLINED\")", escapedBranch, escapedBranch)
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests?q=%s&sort=-updated_on&pagelen=50",
		c.baseURL(), c.Workspace, c.RepoSlug, url.QueryEscape(query))

	// Bitbucket often returns short hashes (12 chars), while the input commit might be full SHA (40 chars).
	// We check for prefix match in both directions to be safe.
	match := func(sha1, sha2 string) bool {
		if sha1 == "" || sha2 == "" {
			return false
		}
		return strings.HasPrefix(sha1, sha2) || strings.HasPrefix(sha2, sha1)
	}

	filter := func(pr PR) bool {
		return match(commit, pr.Source.Commit.ShortHash) || match(commit, pr.MergeCommit.ShortHash)
	}

	return c.getPullRequests(ctx, url, filter)
}

// getPullRequests handles pagination and fetching PRs from a given URL.
// Optional filter function can be provided to filter results client-side.
func (c *Client) getPullRequests(ctx context.Context, url string, filter func(PR) bool) ([]PR, error) {
	var matched []PR

	for url != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/json")

		if c.HTTPClient == nil {
			c.HTTPClient = &http.Client{}
		}

		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		// We explicitly close the body here to avoid accumulating open connections in the loop
		// which would happen if we used defer.
		err = errors.L(err, resp.Body.Close()).AsError()

		if err != nil {
			return nil, errors.E(err, "reading response body")
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d from %s (%s)", resp.StatusCode, url, data)
		}

		var prResp PullRequestResponse
		err = json.Unmarshal(data, &prResp)
		if err != nil {
			return nil, errors.E(err, "unmarshaling PR list")
		}

		for _, pr := range prResp.Values {
			if filter == nil || filter(pr) {
				matched = append(matched, pr)
			}
		}

		url = prResp.Next
	}

	return matched, nil
}

// GetPullRequest fetches a pull request by its ID.
func (c *Client) GetPullRequest(ctx context.Context, id int) (pr PR, err error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d",
		c.baseURL(), c.Workspace, c.RepoSlug, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return PR{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PR{}, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return PR{}, errors.E(err, "reading response body")
	}

	if resp.StatusCode != http.StatusOK {
		return PR{}, fmt.Errorf("unexpected status code: %d (%s)", resp.StatusCode, data)
	}

	err = json.Unmarshal(data, &pr)
	if err != nil {
		return PR{}, errors.E(err, "unmarshaling PR")
	}

	return pr, nil
}

// GetCommit fetches a commit by its hash.
func (c *Client) GetCommit(ctx context.Context, commit string) (Commit, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/commit/%s",
		c.baseURL(), c.Workspace, c.RepoSlug, commit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Commit{}, errors.E(err, "reading response body")
	}

	if resp.StatusCode != http.StatusOK {
		return Commit{}, fmt.Errorf("unexpected status code: %d (%s)", resp.StatusCode, data)
	}

	var commitData Commit
	err = json.Unmarshal(data, &commitData)
	if err != nil {
		return Commit{}, errors.E(err, "unmarshaling commit")
	}

	return commitData, nil
}

// GetUser fetches the user by its UUID.
func (c *Client) GetUser(ctx context.Context, uuid string) (u User, err error) {
	url := fmt.Sprintf("%s/users/%s",
		c.baseURL(), uuid)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return User{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return User{}, errors.E(err, "reading response body")
	}

	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("unexpected status code: %d (%s)", resp.StatusCode, data)
	}

	err = json.Unmarshal(data, &u)
	if err != nil {
		return User{}, errors.E(err, "unmarshaling user")
	}

	return u, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}

	c.BaseURL = "https://api.bitbucket.org/2.0"
	return c.BaseURL
}
