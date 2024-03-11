// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

type (
	// Resource is the interface used to represent resource entities.
	Resource interface {
		Validate() error
	}

	// WellKnown is the well-known payload for cli.json.
	WellKnown struct {
		RequiredVersion string `json:"required_version"`
	}

	// MemberOrganizations is a list of organizations associated with the member.
	MemberOrganizations []MemberOrganization

	// User represents the signed in user information.
	User struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		JobTitle    string `json:"job_title"`
		UUID        UUID   `json:"user_uuid"`
	}

	// MemberOrganization represents the organization associated with the member.
	MemberOrganization struct {
		MemberID    int64  `json:"member_id,omitempty"`
		Name        string `json:"org_name"`
		DisplayName string `json:"org_display_name"`
		Domain      string `json:"org_domain"`
		UUID        UUID   `json:"org_uuid"`
		Role        string `json:"role,omitempty"`
		Status      string `json:"status"`
	}

	// StackObject represents a stack object in the Terramate Cloud.
	StackObject struct {
		ID int64 `json:"stack_id"`
		Stack
		Status           stack.Status      `json:"status"`
		DeploymentStatus deployment.Status `json:"deployment_status"`
		DriftStatus      drift.Status      `json:"drift_status"`

		// readonly fields
		CreatedAt *time.Time `json:"created_at,omitempty"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
		SeenAt    *time.Time `json:"seen_at,omitempty"`
	}

	// Stack represents the stack as defined by the user HCL code.
	Stack struct {
		Repository      string   `json:"repository"`
		DefaultBranch   string   `json:"default_branch"`
		Path            string   `json:"path"`
		MetaID          string   `json:"meta_id"`
		MetaName        string   `json:"meta_name,omitempty"`
		MetaDescription string   `json:"meta_description,omitempty"`
		MetaTags        []string `json:"meta_tags,omitempty"`
	}

	// ChangesetDetails represents the details of a changeset (e.g. the terraform plan).
	ChangesetDetails struct {
		Provisioner    string `json:"provisioner"`
		ChangesetASCII string `json:"changeset_ascii,omitempty"`
		ChangesetJSON  string `json:"changeset_json,omitempty"`
	}

	// StacksResponse represents the stacks object response.
	StacksResponse struct {
		Stacks     []StackObject   `json:"stacks"`
		Pagination PaginatedResult `json:"paginated_result"`
	}

	// DeploymentStackRequest represents the stack object of the request payload
	// type for the creation of stack deployments.
	DeploymentStackRequest struct {
		Stack

		CommitSHA         string            `json:"commit_sha,omitempty"`
		DeploymentURL     string            `json:"deployment_url,omitempty"`
		DeploymentStatus  deployment.Status `json:"deployment_status,omitempty"`
		DeploymentCommand string            `json:"deployment_cmd"`
	}

	// DeploymentStackResponse represents the deployment creation response item.
	DeploymentStackResponse struct {
		StackID     int64             `json:"stack_id"`
		StackMetaID string            `json:"meta_id"`
		Status      deployment.Status `json:"status"`
	}

	// DeploymentStacksResponse represents the list of DeploymentStackResponse.
	DeploymentStacksResponse []DeploymentStackResponse

	// DeploymentStackRequests is a list of DeploymentStacksRequest.
	DeploymentStackRequests []DeploymentStackRequest

	// DeploymentStacksPayloadRequest is the request payload for the creation of stack deployments.
	DeploymentStacksPayloadRequest struct {
		ReviewRequest *ReviewRequest          `json:"review_request,omitempty"`
		Stacks        DeploymentStackRequests `json:"stacks"`
		Workdir       project.Path            `json:"workdir"`
		Metadata      *DeploymentMetadata     `json:"metadata,omitempty"`
	}

	// Drift represents the drift information for a given stack.
	Drift struct {
		ID       int64               `json:"id"`
		Status   drift.Status        `json:"status"`
		Details  *ChangesetDetails   `json:"drift_details,omitempty"`
		Metadata *DeploymentMetadata `json:"metadata,omitempty"`
	}

	// Drifts is a list of drift.
	Drifts []Drift

	// DriftsStackPayloadResponse is the payload returned when listing stack drifts.
	DriftsStackPayloadResponse struct {
		Drifts     Drifts          `json:"drifts"`
		Pagination PaginatedResult `json:"paginated_result"`
	}

	// DriftStackPayloadRequest is the payload for the drift sync.
	DriftStackPayloadRequest struct {
		Stack      Stack               `json:"stack"`
		Status     drift.Status        `json:"drift_status"`
		Details    *ChangesetDetails   `json:"drift_details,omitempty"`
		Metadata   *DeploymentMetadata `json:"metadata,omitempty"`
		StartedAt  *time.Time          `json:"started_at,omitempty"`
		FinishedAt *time.Time          `json:"finished_at,omitempty"`
		Command    []string            `json:"command"`
	}

	// DriftStackPayloadRequests is a list of DriftStackPayloadRequest
	DriftStackPayloadRequests []DriftStackPayloadRequest

	// DeploymentMetadata stores the metadata available in the target platform.
	// It's marshaled as a flat hashmap of values.
	// Note: no sensitive information must be stored here because it could be logged.
	DeploymentMetadata struct {
		GitMetadata

		GithubPullRequestAuthorLogin      string `json:"github_pull_request_author_login,omitempty"`
		GithubPullRequestAuthorAvatarURL  string `json:"github_pull_request_author_avatar_url,omitempty"`
		GithubPullRequestAuthorGravatarID string `json:"github_pull_request_author_gravatar_id,omitempty"`

		GithubPullRequestURL            string `json:"github_pull_request_url,omitempty"`
		GithubPullRequestNumber         int    `json:"github_pull_request_number,omitempty"`
		GithubPullRequestTitle          string `json:"github_pull_request_title,omitempty"`
		GithubPullRequestDescription    string `json:"github_pull_request_description,omitempty"`
		GithubPullRequestState          string `json:"github_pull_request_state,omitempty"`
		GithubPullRequestMergeCommitSHA string `json:"github_pull_request_merge_commit_sha,omitempty"`

		GithubPullRequestHeadLabel            string `json:"github_pull_request_head_label,omitempty"`
		GithubPullRequestHeadRef              string `json:"github_pull_request_head_ref,omitempty"`
		GithubPullRequestHeadSHA              string `json:"github_pull_request_head_sha,omitempty"`
		GithubPullRequestHeadAuthorLogin      string `json:"github_pull_request_head_author_login,omitempty"`
		GithubPullRequestHeadAuthorAvatarURL  string `json:"github_pull_request_head_author_avatar_url,omitempty"`
		GithubPullRequestHeadAuthorGravatarID string `json:"github_pull_request_head_author_gravatar_id,omitempty"`

		GithubPullRequestBaseLabel            string `json:"github_pull_request_base_label,omitempty"`
		GithubPullRequestBaseRef              string `json:"github_pull_request_base_ref,omitempty"`
		GithubPullRequestBaseSHA              string `json:"github_pull_request_base_sha,omitempty"`
		GithubPullRequestBaseAuthorLogin      string `json:"github_pull_request_base_author_login,omitempty"`
		GithubPullRequestBaseAuthorAvatarURL  string `json:"github_pull_request_base_author_avatar_url,omitempty"`
		GithubPullRequestBaseAuthorGravatarID string `json:"github_pull_request_base_author_gravatar_id,omitempty"`

		GithubPullRequestCreatedAt *time.Time `json:"github_pull_request_created_at,omitempty"`
		GithubPullRequestUpdatedAt *time.Time `json:"github_pull_request_updated_at,omitempty"`
		GithubPullRequestClosedAt  *time.Time `json:"github_pull_request_closed_at,omitempty"`
		GithubPullRequestMergedAt  *time.Time `json:"github_pull_request_merged_at,omitempty"`

		GithubCommitVerified       *bool  `json:"github_commit_verified,omitempty"`
		GithubCommitVerifiedReason string `json:"github_commit_verified_reason,omitempty"`

		GithubCommitTitle            string     `json:"github_commit_title,omitempty"`
		GithubCommitDescription      string     `json:"github_commit_description,omitempty"`
		GithubCommitAuthorLogin      string     `json:"github_commit_author_login,omitempty"`
		GithubCommitAuthorAvatarURL  string     `json:"github_commit_author_avatar_url,omitempty"`
		GithubCommitAuthorGravatarID string     `json:"github_commit_author_gravatar_id,omitempty"`
		GithubCommitAuthorGitName    string     `json:"github_commit_author_git_name,omitempty"`
		GithubCommitAuthorGitEmail   string     `json:"github_commit_author_git_email,omitempty"`
		GithubCommitAuthorGitDate    *time.Time `json:"github_commit_author_git_date,omitempty"`

		GithubCommitCommitterLogin      string     `json:"github_commit_committer_login,omitempty"`
		GithubCommitCommitterAvatarURL  string     `json:"github_commit_committer_avatar_url,omitempty"`
		GithubCommitCommitterGravatarID string     `json:"github_commit_committer_gravatar_id,omitempty"`
		GithubCommitCommitterGitName    string     `json:"github_commit_committer_git_name,omitempty"`
		GithubCommitCommitterGitEmail   string     `json:"github_commit_committer_git_email,omitempty"`
		GithubCommitCommitterGitDate    *time.Time `json:"github_commit_committer_git_date,omitempty"`

		GithubActionsDeploymentBranch      string `json:"github_actions_deployment_branch,omitempty"`
		GithubActionsDeploymentTriggeredBy string `json:"github_actions_triggered_by,omitempty"`
		GithubActionsRunID                 string `json:"github_actions_run_id,omitempty"`
		GithubActionsRunAttempt            string `json:"github_actions_run_attempt,omitempty"`
		GithubActionsWorkflowName          string `json:"github_actions_workflow_name,omitempty"`
		GithubActionsWorkflowRef           string `json:"github_actions_workflow_ref,omitempty"`
	}

	// GitMetadata are the git related metadata.
	GitMetadata struct {
		GitCommitSHA         string     `json:"git_commit_sha,omitempty"`
		GitCommitAuthorName  string     `json:"git_commit_author_name,omitempty"`
		GitCommitAuthorEmail string     `json:"git_commit_author_email,omitempty"`
		GitCommitAuthorTime  *time.Time `json:"git_commit_author_time,omitempty"`
		GitCommitTitle       string     `json:"git_commit_title,omitempty"`
		GitCommitDescription string     `json:"git_commit_description,omitempty"`
	}

	// ReviewRequest is the review_request object.
	ReviewRequest struct {
		Platform              string     `json:"platform"`
		Repository            string     `json:"repository"`
		CommitSHA             string     `json:"commit_sha"`
		Number                int        `json:"number"`
		Title                 string     `json:"title"`
		Description           string     `json:"description"`
		URL                   string     `json:"url"`
		Labels                []Label    `json:"labels,omitempty"`
		Reviewers             Reviewers  `json:"reviewers,omitempty"`
		Status                string     `json:"status"`
		Draft                 bool       `json:"draft"`
		ReviewDecision        string     `json:"review_decision"`
		ChangesRequestedCount int        `json:"changes_requested_count"`
		ApprovedCount         int        `json:"approved_count"`
		ChecksTotalCount      int        `json:"checks_total_count"`
		ChecksFailureCount    int        `json:"checks_failure_count"`
		ChecksSuccessCount    int        `json:"checks_success_count"`
		UpdatedAt             *time.Time `json:"updated_at,omitempty"`
	}

	// Label of a review request.
	Label struct {
		Name        string `json:"name"`
		Color       string `json:"color,omitempty"`
		Description string `json:"description,omitempty"`
	}

	// Reviewer is the user's reviewer of a Pull/Merge Request.
	Reviewer struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url,omitempty"`
	}

	// Reviewers is a list of reviewers.
	Reviewers []Reviewer

	// UpdateDeploymentStack is the request payload item for updating the deployment status.
	UpdateDeploymentStack struct {
		StackID int64             `json:"stack_id"`
		Status  deployment.Status `json:"status"`
		Details *ChangesetDetails `json:"changeset_details,omitempty"`
	}

	// UpdateDeploymentStacks is the request payload for updating the deployment status.
	UpdateDeploymentStacks struct {
		Stacks []UpdateDeploymentStack `json:"stacks"`
	}

	// CommandLogs represents a batch of log messages.
	CommandLogs []*CommandLog

	// LogChannel is an enum-like type for the output channels supported.
	LogChannel int

	// CommandLog represents a single log message.
	CommandLog struct {
		Line      int64      `json:"log_line"`
		Timestamp *time.Time `json:"timestamp"`
		Channel   LogChannel `json:"channel"`
		Message   string     `json:"message"`
	}

	// ReviewRequestResponsePayload is the review request response payload.
	ReviewRequestResponsePayload struct {
		ReviewRequests ReviewRequestResponses `json:"review_requests"`
		Pagination     PaginatedResult        `json:"paginated_result"`
	}

	// ReviewRequestResponses is a list of review request responses.
	ReviewRequestResponses []ReviewRequestResponse

	// ReviewRequestResponse is the response payload for the review request creation.
	ReviewRequestResponse struct {
		ID        int64  `json:"review_request_id"`
		CommitSHA string `json:"commit_sha"`
		Number    int    `json:"number"`
	}

	// PaginatedResult represents the pagination object.
	PaginatedResult struct {
		Total   int64 `json:"total"`
		Page    int64 `json:"page"`
		PerPage int64 `json:"per_page"`
	}

	// UUID represents an UUID string.
	UUID string
)

const (
	unknownLogChannel LogChannel = iota
	StdoutLogChannel             // StdoutLogChannel is the stdout channel
	StderrLogChannel             // StderrLogChannel is the stderr channel
)

var (
	// compile-time checks to ensure resource entities implement the Resource iface.
	_ = Resource(WellKnown{})
	_ = Resource(User{})
	_ = Resource(MemberOrganization{})
	_ = Resource(MemberOrganizations{})
	_ = Resource(StackObject{})
	_ = Resource(StacksResponse{})
	_ = Resource(DeploymentStackRequest{})
	_ = Resource(DeploymentStackRequests{})
	_ = Resource(DeploymentStacksPayloadRequest{})
	_ = Resource(DeploymentStackResponse{})
	_ = Resource(DeploymentStacksResponse{})
	_ = Resource(UpdateDeploymentStack{})
	_ = Resource(UpdateDeploymentStacks{})
	_ = Resource(ReviewRequest{})
	_ = Resource(Reviewer{})
	_ = Resource(Reviewers{})
	_ = Resource(Label{})
	_ = Resource(Drifts{})
	_ = Resource(DriftStackPayloadRequest{})
	_ = Resource(DriftStackPayloadRequests{})
	_ = Resource(ChangesetDetails{})
	_ = Resource(CommandLogs{})
	_ = Resource(CommandLog{})
	_ = Resource(CreatePreviewPayloadRequest{})
	_ = Resource(CreatePreviewResponse{})
	_ = Resource(UpdateStackPreviewPayloadRequest{})
	_ = Resource(ReviewRequestResponse{})
	_ = Resource(ReviewRequestResponses{})
	_ = Resource(ReviewRequestResponsePayload{})
	_ = Resource(EmptyResponse(""))
)

// Validate the review request response payload.
func (rr ReviewRequestResponsePayload) Validate() error {
	if err := rr.ReviewRequests.Validate(); err != nil {
		return err
	}
	return rr.Pagination.Validate()
}

// Validate the ReviewRequestResponse object.
func (rr ReviewRequestResponse) Validate() error {
	if rr.ID == 0 {
		return errors.E(`missing "review_request_id" field`)
	}
	return nil
}

// Validate the list of review request responses.
func (rrs ReviewRequestResponses) Validate() error {
	return validateResourceList(rrs...)
}

// String is a human readable list of organizations associated with a user.
func (orgs MemberOrganizations) String() string {
	str := make([]string, len(orgs))
	for i, org := range orgs {
		str[i] = fmt.Sprintf("%s (%s)", org.DisplayName, org.Name)
	}

	return strings.Join(str, ", ")
}

// Validate the well-known payload.
func (wk WellKnown) Validate() error {
	if wk.RequiredVersion == "" {
		return errors.E(`missing "required_version" field`)
	}
	return nil
}

// Validate if the user has the Terramate CLI required fields.
func (u User) Validate() error {
	if u.Email == "" {
		return errors.E(`missing "email" field.`)
	}
	if u.DisplayName == "" {
		return errors.E(`missing "display_name" field`)
	}
	return nil
}

// Validate if the organization list is valid.
func (orgs MemberOrganizations) Validate() error {
	return validateResourceList(orgs...)
}

// Validate checks if at least the fields required by Terramate CLI are set.
func (org MemberOrganization) Validate() error {
	if org.Name == "" {
		return errors.E(`missing "name" field`)
	}
	if org.UUID == "" {
		return errors.E(`missing "org_uuid" field`)
	}
	return nil
}

// Validate the stack entity.
func (stack StackObject) Validate() error {
	if stack.MetaID == "" {
		return errors.E(`missing "meta_id" field`)
	}
	if stack.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	return nil
}

// Validate the StacksResponse object.
func (stacksResp StacksResponse) Validate() error {
	for _, st := range stacksResp.Stacks {
		err := st.Validate()
		if err != nil {
			return err
		}
	}
	return stacksResp.Pagination.Validate()
}

// Validate the deployment stack request.
func (d DeploymentStackRequest) Validate() error {
	if err := d.Stack.Validate(); err != nil {
		return err
	}
	if d.DeploymentCommand == "" {
		return errors.E(`missing "deployment_cmd" field`)
	}
	return nil
}

// Validate the stack object.
func (s Stack) Validate() error {
	if s.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	if s.DefaultBranch == "" {
		return errors.E(`missing "default_branch" field`)
	}
	if s.Path == "" {
		return errors.E(`missing "path" field`)
	}
	if s.MetaID == "" {
		return errors.E(`missing "meta_id" field`)
	}
	if strings.ToLower(s.MetaID) != s.MetaID {
		return errors.E(`"meta_id" requires a lowercase string but %s provided`, s.MetaID)
	}
	return nil
}

// Validate a drift.
func (d Drift) Validate() error {
	if err := d.Status.Validate(); err != nil {
		return err
	}
	if d.Details != nil {
		return d.Details.Validate()
	}
	return nil
}

// Validate a list of drifts.
func (ds Drifts) Validate() error {
	return validateResourceList[Drift](ds...)
}

// Validate the drift request payload.
func (d DriftStackPayloadRequest) Validate() error {
	if err := d.Stack.Validate(); err != nil {
		return err
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}
	if d.Details != nil {
		return d.Details.Validate()
	}
	if d.Metadata != nil {
		err := d.Metadata.Validate()
		if err != nil {
			return err
		}
	}

	if d.Details != nil {
		err := d.Details.Validate()
		if err != nil {
			return err
		}
	}

	return d.Status.Validate()
}

// Validate the list of drift requests.
func (ds DriftStackPayloadRequests) Validate() error { return validateResourceList(ds...) }

// Validate the drifts list response payload.
func (ds DriftsStackPayloadResponse) Validate() error {
	if err := ds.Pagination.Validate(); err != nil {
		return err
	}
	return validateResourceList(ds.Drifts...)
}

// Validate the drift details.
func (ds ChangesetDetails) Validate() error {
	if ds.Provisioner == "" && ds.ChangesetASCII == "" && ds.ChangesetJSON == "" {
		// TODO: backend returns the `details` object even if it was not synchronized.
		return nil
	}
	if ds.Provisioner == "" {
		return errors.E(`field "provisioner" is required`)
	}
	if ds.ChangesetASCII == "" && ds.ChangesetJSON == "" {
		return errors.E(`"changeset_ascii" or "changeset_json" must be set`)
	}
	return nil
}

// Validate the list of deployment stack requests.
func (d DeploymentStackRequests) Validate() error { return validateResourceList(d...) }

// Validate the deployment stack payload.
func (d DeploymentStacksPayloadRequest) Validate() error {
	if d.ReviewRequest != nil {
		err := d.ReviewRequest.Validate()
		if err != nil {
			return err
		}
	}
	if d.Metadata != nil {
		err := d.Metadata.Validate()
		if err != nil {
			return err
		}
	}
	if d.Workdir.String() == "" {
		return errors.E(`missing "workdir" field`)
	}
	return validateResourceList(d.Stacks)
}

// Validate the metadata.
func (m DeploymentMetadata) Validate() error {
	return nil
}

// Validate the deployment stack response.
func (d DeploymentStackResponse) Validate() error {
	return d.Status.Validate()
}

// Validate the DeploymentReviewRequest object.
func (rr ReviewRequest) Validate() error {
	if rr.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	return validateResourceList(rr.Labels...)
}

// Validate the label.
func (l Label) Validate() error {
	if l.Name == "" {
		return errors.E(`missing "name" field`)
	}
	return nil
}

// Validate the reviewer.
func (r Reviewer) Validate() error {
	if r.Login == "" {
		return errors.E(`missing "login" field`)
	}
	return nil
}

// Validate the reviewers list.
func (rs Reviewers) Validate() error { return validateResourceList(rs...) }

// Validate the UpdateDeploymentStack object.
func (d UpdateDeploymentStack) Validate() error {
	if d.StackID == 0 {
		return errors.E(`invalid "stack_id" of value %d`, d.StackID)
	}
	return d.Status.Validate()
}

// Validate the list of UpdateDeploymentStack.
func (ds UpdateDeploymentStacks) Validate() error { return validateResourceList(ds.Stacks...) }

// Validate the list of deployment stacks response.
func (ds DeploymentStacksResponse) Validate() error { return validateResourceList(ds...) }

// Validate a command log.
func (l CommandLog) Validate() error {
	if l.Channel == unknownLogChannel {
		return errors.E(`missing "channel" field`)
	}
	if l.Timestamp == nil {
		return errors.E(`missing "timestamp" field`)
	}
	return nil
}

// Validate a list of command logs.
func (ls CommandLogs) Validate() error { return validateResourceList(ls...) }
func validateResourceList[T Resource](resources ...T) error {
	for _, resource := range resources {
		err := resource.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate the "paginate_result" field.
func (p PaginatedResult) Validate() error {
	if p.Page == 0 {
		return errors.E(`field "paginated_result.page" is zero or absent`)
	}
	return nil
}

// EmptyResponse is used to represent an empty string response.
type EmptyResponse string

// Validate that content is empty.
func (s EmptyResponse) Validate() error {
	if s == "" {
		return nil
	}
	return errors.E("unexpected non-empty string")
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (c *LogChannel) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	switch str {
	case "stdout":
		*c = StdoutLogChannel
	case "stderr":
		*c = StderrLogChannel
	default:
		return errors.E("unrecognized log channel: %s", str)
	}
	return nil
}

// MarshalJSON implements the [json.Marshaler] interface.
func (c *LogChannel) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// String returns the channel name.
func (c LogChannel) String() string {
	if c == StdoutLogChannel {
		return "stdout"
	}
	if c == StderrLogChannel {
		return "stderr"
	}
	return "unknown"
}
