// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/printer"
)

const (
	// ErrNotFound indicates the resource does not exists.
	ErrNotFound errors.Kind = "resource not found (HTTP Status: 404)"
)

type (
	// Client is a Gitlab API HTTP client wrapper.
	Client struct {
		// BaseURL is the base URL used to construct the final URL of endpoints.
		// If not set, it uses CI_API_V4_URL environment variable.
		BaseURL string

		// HTTPClient sets the HTTP client used and then allows for advanced
		// connection reuse schemes. If not set, a new http.Client is used.
		HTTPClient *http.Client

		// Token is the Gitlab token (usually provided by the GITLAB_TOKEN environment variable.
		Token string

		ProjectID int64
		Group     string
		Project   string
	}

	// MRs is a list of Gitlab Merge Requests.
	MRs []MR

	// MR is a Gitlab Merge Request.
	MR struct {
		ID                  int      `json:"id"`
		IID                 int      `json:"iid"`
		ProjectID           int      `json:"project_id"`
		Title               string   `json:"title"`
		Description         string   `json:"description,omitempty"`
		State               string   `json:"state"`
		CreatedAt           string   `json:"created_at,omitempty"`
		UpdatedAt           string   `json:"updated_at,omitempty"`
		TargetBranch        string   `json:"target_branch"`
		SourceBranch        string   `json:"source_branch"`
		Upvotes             int      `json:"upvotes"`
		Downvotes           int      `json:"downvotes"`
		Author              User     `json:"author"`
		Labels              []string `json:"labels"`
		Draft               bool     `json:"draft"`
		WorkInProgress      bool     `json:"work_in_progress"`
		SHA                 string   `json:"sha"`
		MergeStatus         string   `json:"merge_status"`
		DetailedMergeStatus string   `json:"detailed_merge_status"`
		WebURL              string   `json:"web_url"`
		Assignees           []User   `json:"assignees"`

		// fields below are available but not used:
		// - assignee
		// - milestone
		// - merge_when_pipeline_succeeds
		// - merge_commit_sha
		// - squash_commit_sha
		// - user_notes_count
		// - discussion_locked
		// - should_remove_source_branch
		// - force_remove_source_branch
		// - time_stats.time_estimate
		// - time_statis.total_time_spent
		// - time_stats.human_time_estimate
		// - time_stats.human_total_time_spent
	}

	// User is a Gitlab user.
	User struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		WebURL    string `json:"web_url"`
		AvatarURL string `json:"avatar_url"`
		State     string `json:"state"`
	}

	// MRReviewer is a Merge Request reviewer.
	MRReviewer struct {
		User      *User      `json:"user"`
		State     string     `json:"state"`
		CreatedAt *time.Time `json:"created_at"`
	}
)

// MRForCommit returns the with first Merge Requests associated with the provided commit SHA.
func (c *Client) MRForCommit(ctx context.Context, sha string) (mr MR, found bool, err error) {
	var url string
	if c.ProjectID != 0 {
		url = fmt.Sprintf("%s/projects/%d/repository/commits/%s/merge_requests?per_page=%d", c.baseURL(), c.ProjectID, sha, 1)
	} else {
		url = fmt.Sprintf("%s/projects/%s/repository/commits/%s/merge_requests?per_page=%d", c.baseURL(), c.projectRef(), sha, 1)
	}
	data, err := c.doGet(ctx, url)
	if err != nil {
		return MR{}, false, err
	}
	var mrs MRs
	err = json.Unmarshal(data, &mrs)
	if err != nil {
		return MR{}, false, errors.E(err, "unmarshaling MR list")
	}
	if len(mrs) == 0 {
		return MR{}, false, nil
	}
	return mrs[0], true, nil
}

// MRReviewers returns the the reviewers for the given MR.
func (c *Client) MRReviewers(ctx context.Context, mrIID int) ([]MRReviewer, error) {
	var url string
	if c.ProjectID != 0 {
		url = fmt.Sprintf("%s/projects/%d/merge_requests/%d/reviewers", c.baseURL(), c.ProjectID, mrIID)
	} else {
		url = fmt.Sprintf("%s/projects/%s/merge_requests/%d/reviewers", c.baseURL(), c.projectRef(), mrIID)
	}
	data, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var reviewers []MRReviewer
	err = json.Unmarshal(data, &reviewers)
	if err != nil {
		return nil, errors.E(err, "unmarshaling reviewers list")
	}
	return reviewers, nil
}

// MRParticipants returns the the participants for the given MR.
func (c *Client) MRParticipants(ctx context.Context, mrIID int) ([]User, error) {
	var url string
	if c.ProjectID != 0 {
		url = fmt.Sprintf("%s/projects/%d/merge_requests/%d/participants", c.baseURL(), c.ProjectID, mrIID)
	} else {
		url = fmt.Sprintf("%s/projects/%s/merge_requests/%d/participants", c.baseURL(), c.projectRef(), mrIID)
	}
	data, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var participants []User
	err = json.Unmarshal(data, &participants)
	if err != nil {
		return nil, errors.E(err, "unmarshaling participants list")
	}
	return participants, nil
}

func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.E(err, "creating request")
	}

	if c.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.Token)
	} else {
		printer.Stderr.Warn("GITLAB_TOKEN is not set")
	}

	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.E(err, "requesting GET %s", url)
	}

	defer func() {
		err = errors.L(err, resp.Body.Close()).AsError()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.E(err, "reading response body")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.E(ErrNotFound, "retrieving %s", url)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.E("unexpected status code: %s while getting %s", resp.Status, url)
	}
	return data, nil
}

func (c *Client) projectRef() string {
	return url.QueryEscape(fmt.Sprintf("%s/%s", c.Group, c.Project))
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}

	c.BaseURL = os.Getenv("CI_API_V4_URL")
	if c.BaseURL == "" {
		c.BaseURL = "https://gitlab.com/api/v4"
	}
	return c.BaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	return c.HTTPClient
}
