// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package resources contains the resource entities used in the Terramate Cloud API.
package resources

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/metadata"
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/stack"
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

	// SingleSignOnDetailResponse the response payload for the /v1/organizations/name/<name> endpoint.
	SingleSignOnDetailResponse struct {
		EnterpriseOrgID string `json:"enterprise_org_id"`
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
		Target          string   `json:"target,omitempty"`
		FromTarget      string   `json:"from_target,omitempty"`
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
		Serial         *int64 `json:"serial,omitempty"`
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
		StackID     int64  `json:"stack_id"`
		StackMetaID string `json:"meta_id"`
		// TODO(snk): The target in the response is not handled yet. This needs to happen once we
		// support creating deployments with the same stack/meta_id on multiple targets.
		Target string            `json:"target"`
		Status deployment.Status `json:"status"`
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
		UUID     UUID                `json:"uuid,omitempty"`
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

	// DriftCheckRunStartPayloadRequest is the payload for starting drift sync.
	DriftCheckRunStartPayloadRequest struct {
		Stack      Stack               `json:"stack"`
		Metadata   *DeploymentMetadata `json:"metadata,omitempty"`
		StartedAt  *time.Time          `json:"started_at,omitempty"`
		FinishedAt *time.Time          `json:"finished_at,omitempty"`
		Command    []string            `json:"command"`
	}

	// DriftCheckRunStartResponse represents the drift creation response.
	DriftCheckRunStartResponse struct {
		DriftUUID UUID `json:"uuid"`
	}

	// UpdateDriftPayloadRequest is the request payload for updating a drift.
	UpdateDriftPayloadRequest struct {
		Status    drift.Status      `json:"status"`
		Changeset *ChangesetDetails `json:"changeset,omitempty"`
		UpdatedAt time.Time         `json:"updated_at"`
	}

	// DriftWithStack is the drift API object. Not used by Terramate CLI but only by the test server by now.
	DriftWithStack struct {
		Drift
		Stack
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}

	// DriftsWithStacks is a list of drifts with stacks.
	DriftsWithStacks []DriftWithStack

	// CreatePreviewPayloadRequest is the request payload for the creation of
	// stack deployments.
	CreatePreviewPayloadRequest struct {
		CommitSHA       string              `json:"commit_sha"`
		PushedAt        int64               `json:"pushed_at"`
		Technology      string              `json:"technology"`
		TechnologyLayer string              `json:"technology_layer"`
		ReviewRequest   *ReviewRequest      `json:"review_request,omitempty"`
		Metadata        *DeploymentMetadata `json:"metadata,omitempty"`
		Stacks          PreviewStacks       `json:"stacks"`
	}

	// PreviewStacks is a list of stack objects for the request payload
	PreviewStacks []PreviewStack

	// PreviewStack represents the stack object of the request payload
	// type for the creation of stack deployments.
	PreviewStack struct {
		Stack

		PreviewStatus preview.StackStatus `json:"preview_status"`
		Cmd           []string            `json:"cmd,omitempty"`
	}

	// CreatePreviewResponse represents the deployment creation response item.
	CreatePreviewResponse struct {
		PreviewID string                `json:"preview_id"`
		Stacks    ResponsePreviewStacks `json:"stacks"`
	}

	// UpdateStackPreviewPayloadRequest is the request payload for the update of
	// stack previews.
	UpdateStackPreviewPayloadRequest struct {
		Status           string            `json:"status"`
		ChangesetDetails *ChangesetDetails `json:"changeset_details,omitempty"`
	}

	// ResponsePreviewStacks is a list of stack objects in the response payload
	ResponsePreviewStacks []ResponsePreviewStack

	// ResponsePreviewStack represents a specific stack in the preview response.
	ResponsePreviewStack struct {
		MetaID         string `json:"meta_id"`
		StackPreviewID string `json:"stack_preview_id"`
	}

	// DriftStackPayloadRequests is a list of DriftStackPayloadRequest
	DriftStackPayloadRequests []DriftCheckRunStartPayloadRequest

	// DeploymentMetadata stores the metadata available in the target platform.
	// It's marshaled as a flat hashmap of values.
	// Note: no sensitive information must be stored here because it could be logged.
	DeploymentMetadata struct {
		GitMetadata
		GithubMetadata
		GitlabMetadata
		BitbucketMetadata
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

	// GithubMetadata is the GitHub related metadata
	GithubMetadata struct {
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
		GithubActionsDeploymentActorID     string `json:"github_actions_deployment_actor_id,omitempty"`
		GithubActionsDeploymentActor       string `json:"github_actions_deployment_actor,omitempty"`
		GithubActionsDeploymentTriggeredBy string `json:"github_actions_triggered_by,omitempty"`
		GithubActionsRunID                 string `json:"github_actions_run_id,omitempty"`
		GithubActionsRunAttempt            string `json:"github_actions_run_attempt,omitempty"`
		GithubActionsWorkflowName          string `json:"github_actions_workflow_name,omitempty"`
		GithubActionsWorkflowRef           string `json:"github_actions_workflow_ref,omitempty"`
		GithubActionsServerURL             string `json:"github_actions_server_url,omitempty"` // GITHUB_SERVER_URL

		GithubCommit      *metadata.GithubCommit      `json:"github_commit,omitempty"`
		GithubPullRequest *metadata.GithubPullRequest `json:"github_pull_request,omitempty"`
	}

	// GitlabMetadata holds the Gitlab specific metadata.
	GitlabMetadata struct {
		GitlabMergeRequestAuthorID        int    `json:"gitlab_merge_request_author_id,omitempty"`
		GitlabMergeRequestAuthorName      string `json:"gitlab_merge_request_author_name,omitempty"`
		GitlabMergeRequestAuthorWebURL    string `json:"gitlab_merge_request_author_web_url,omitempty"`
		GitlabMergeRequestAuthorUsername  string `json:"gitlab_merge_request_author_username,omitempty"`
		GitlabMergeRequestAuthorAvatarURL string `json:"gitlab_merge_request_author_avatar_url,omitempty"`
		GitlabMergeRequestAuthorState     string `json:"gitlab_merge_request_author_state,omitempty"`

		GitlabMergeRequestID           int    `json:"gitlab_merge_request_id,omitempty"`
		GitlabMergeRequestIID          int    `json:"gitlab_merge_request_iid,omitempty"`
		GitlabMergeRequestState        string `json:"gitlab_merge_request_state,omitempty"`
		GitlabMergeRequestCreatedAt    string `json:"gitlab_merge_request_created_at,omitempty"`
		GitlabMergeRequestUpdatedAt    string `json:"gitlab_merge_request_updated_at,omitempty"`
		GitlabMergeRequestTargetBranch string `json:"gitlab_merge_request_target_branch,omitempty"`
		GitlabMergeRequestSourceBranch string `json:"gitlab_merge_request_source_branch,omitempty"`
		GitlabMergeRequestMergeStatus  string `json:"gitlab_merge_request_merge_status,omitempty"`
		GitlabMergeRequestWebURL       string `json:"gitlab_merge_request_web_url,omitempty"`

		// CICD
		GitlabCICDJobManual         bool   `json:"gitlab_cicd_job_manual,omitempty"`          // CI_JOB_MANUAL
		GitlabCICDPipelineID        string `json:"gitlab_cicd_pipeline_id,omitempty"`         // CI_PIPELINE_ID
		GitlabCICDPipelineSource    string `json:"gitlab_cicd_pipeline_source,omitempty"`     // CI_PIPELINE_SOURCE
		GitlabCICDPipelineName      string `json:"gitlab_cicd_pipeline_name,omitempty"`       // CI_PIPELINE_NAME
		GitlabCICDPipelineTriggered bool   `json:"gitlan_cicd_pipeline_triggered,omitempty"`  // CI_PIPELINE_TRIGGERED
		GitlabCICDPipelineURL       string `json:"gitlab_cicd_pipeline_url,omitempty"`        // CI_PIPELINE_URL
		GitlabCICDPipelineCreatedAt string `json:"gitlab_cicd_pipeline_created_at,omitempty"` // CI_PIPELINE_CREATED_AT
		GitlabCICDJobID             string `json:"gitlab_cicd_job_id,omitempty"`              // CI_JOB_ID
		GitlabCICDJobName           string `json:"gitlab_cicd_job_name,omitempty"`            // CI_JOB_NAME
		GitlabCICDJobStartedAt      string `json:"gitlab_cicd_job_started_at,omitempty"`      // CI_JOB_STARTED_AT
		GitlabCICDUserEmail         string `json:"gitlab_cicd_user_email,omitempty"`          // GITLAB_USER_EMAIL
		GitlabCICDUserID            string `json:"gitlab_cicd_user_id,omitempty"`             // GITLAB_USER_ID
		GitlabCICDUserName          string `json:"gitlab_cicd_user_name,omitempty"`           // GITLAB_USER_NAME
		GitlabCICDUserLogin         string `json:"gitlab_cicd_user_login,omitempty"`          // GITLAB_USER_LOGIN
		GitlabCICDCommitBranch      string `json:"gitlab_cicd_commit_branch,omitempty"`       // CI_COMMIT_BRANCH
		GitlabCIServerHost          string `json:"gitlab_cicd_server_host,omitempty"`         // CI_SERVER_HOST
		GitlabCIServerURL           string `json:"gitlab_cicd_server_url,omitempty"`          // CI_SERVER_URL

		// either CI_COMMIT_BRANCH or CI_MERGE_REQUEST_SOURCE_BRANCH_NAME
		GitlabCICDBranch string `json:"gitlab_cicd_branch,omitempty"`

		// Only available for merge request pipelines
		GitlabCICDMergeRequestApproved *bool `json:"gitlab_cicd_merge_request_approved,omitempty"` // CI_MERGE_REQUEST_APPROVED

		GitlabMergeRequest *metadata.GitlabMergeRequest `json:"gitlab_merge_request,omitempty"`
	}

	// BitbucketMetadata holds the Bitbucket specific metadata.
	BitbucketMetadata struct {
		BitbucketPipelinesBuildNumber               string `json:"bitbucket_pipelines_build_number,omitempty"`
		BitbucketPipelinesPipelineUUID              string `json:"bitbucket_pipelines_pipeline_uuid,omitempty"`
		BitbucketPipelinesCommit                    string `json:"bitbucket_pipelines_commit,omitempty"`
		BitbucketPipelinesWorkspace                 string `json:"bitbucket_pipelines_workspace,omitempty"`
		BitbucketPipelinesRepoSlug                  string `json:"bitbucket_pipelines_repo_slug,omitempty"`
		BitbucketPipelinesRepoUUID                  string `json:"bitbucket_pipelines_repo_uuid,omitempty"`
		BitbucketPipelinesRepoFullName              string `json:"bitbucket_pipelines_repo_full_name,omitempty"`
		BitbucketPipelinesBranch                    string `json:"bitbucket_pipelines_branch,omitempty"`
		BitbucketPipelinesDestinationBranch         string `json:"bitbucket_pipelines_destination_branch,omitempty"`
		BitbucketPipelinesTag                       string `json:"bitbucket_pipelines_tag,omitempty"` // only available in tag events.
		BitbucketPipelinesStepTriggererUUID         string `json:"bitbucket_pipelines_step_triggerer_uuid,omitempty"`
		BitbucketPipelinesTriggeredByAccountID      string `json:"bitbucket_pipelines_triggered_by_account_id,omitempty"`
		BitbucketPipelinesTriggeredByNickname       string `json:"bitbucket_pipelines_triggered_by_nickname,omitempty"`
		BitbucketPipelinesTriggeredByDisplayName    string `json:"bitbucket_pipelines_triggered_by_display_name,omitempty"`
		BitbucketPipelinesTriggeredByAvatarURL      string `json:"bitbucket_pipelines_triggered_by_avatar_url,omitempty"`
		BitbucketPipelinesParallelStep              string `json:"bitbucket_pipelines_parallel_step,omitempty"`
		BitbucketPipelinesParallelStepCount         string `json:"bitbucket_pipelines_parallel_step_count,omitempty"`
		BitbucketPipelinesPRID                      string `json:"bitbucket_pipelines_pr_id,omitempty"` // only available in PR events.
		BitbucketPipelinesStepUUID                  string `json:"bitbucket_pipelines_step_uuid,omitempty"`
		BitbucketPipelinesDeploymentEnvironment     string `json:"bitbucket_pipelines_deployment_environment,omitempty"`
		BitbucketPipelinesDeploymentEnvironmentUUID string `json:"bitbucket_pipelines_deployment_environment_uuid,omitempty"`
		BitbucketPipelinesProjectKey                string `json:"bitbucket_pipelines_project_key,omitempty"`
		BitbucketPipelinesProjectUUID               string `json:"bitbucket_pipelines_project_uuid,omitempty"`

		BitbucketPullRequest *metadata.BitbucketPullRequest `json:"bitbucket_pull_request,omitempty"`
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
		Author                Author     `json:"author"`
		Reviewers             Reviewers  `json:"reviewers,omitempty"`
		Status                string     `json:"status"`
		Draft                 bool       `json:"draft"`
		ReviewDecision        string     `json:"review_decision,omitempty"`
		ChangesRequestedCount int        `json:"changes_requested_count"`
		ApprovedCount         int        `json:"approved_count"`
		ChecksTotalCount      int        `json:"checks_total_count"`
		ChecksFailureCount    int        `json:"checks_failure_count"`
		ChecksSuccessCount    int        `json:"checks_success_count"`
		CreatedAt             *time.Time `json:"created_at,omitempty"`
		UpdatedAt             *time.Time `json:"updated_at,omitempty"`
		PushedAt              *int64     `json:"pushed_at,omitempty"`
		Branch                string     `json:"branch"`
		BaseBranch            string     `json:"base_branch"`
	}

	// Label of a review request.
	Label struct {
		Name        string `json:"name"`
		Color       string `json:"color,omitempty"`
		Description string `json:"description,omitempty"`
	}

	// Author of the change.
	Author struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url,omitempty"`
		ID        string `json:"id,omitempty"`
	}

	// Reviewer is the user's reviewer of a Pull/Merge Request.
	Reviewer Author

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

	// StatusFilters defines multiple statuses stack filters.
	StatusFilters struct {
		StackStatus      stack.FilterStatus
		DeploymentStatus deployment.FilterStatus
		DriftStatus      drift.FilterStatus
	}

	// StoreOutputRequest is the request payload for storing a output in /v1/store/<org>/outputs endpoint.
	StoreOutputRequest struct {
		Key   StoreOutputKey `json:"key"`
		Value string         `json:"value"`
	}

	// StoreOutputKey is the key field of the StoreOutputRequest payload.
	StoreOutputKey struct {
		OrgUUID     UUID   `json:"org_uuid"`
		Repository  string `json:"repository"`
		StackMetaID string `json:"stack_meta_id"`
		Target      string `json:"target,omitempty"`
		Name        string `json:"name"`
	}

	// StoreOutput represents an output stored in the Terramate Cloud.
	StoreOutput struct {
		ID UUID `json:"id"`

		Key       StoreOutputKey `json:"key"`
		Value     string         `json:"value"`
		CreatedAt time.Time      `json:"created_at"`
		UpdatedAt time.Time      `json:"updated_at"`
	}

	// UUID represents an UUID string.
	UUID string
)

const (
	unknownLogChannel LogChannel = iota
	// StdoutLogChannel is the stdout channel
	StdoutLogChannel
	// StderrLogChannel is the stderr channel
	StderrLogChannel
)

var (
	// compile-time checks to ensure resource entities implement the Resource iface.
	_ = Resource(WellKnown{})
	_ = Resource(SingleSignOnDetailResponse{})
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
	_ = Resource(DriftCheckRunStartPayloadRequest{})
	_ = Resource(DriftStackPayloadRequests{})
	_ = Resource(DriftWithStack{})
	_ = Resource(DriftsWithStacks{})
	_ = Resource(ChangesetDetails{})
	_ = Resource(CommandLogs{})
	_ = Resource(CommandLog{})
	_ = Resource(CreatePreviewPayloadRequest{})
	_ = Resource(CreatePreviewResponse{})
	_ = Resource(DriftCheckRunStartResponse{})
	_ = Resource(UpdateDriftPayloadRequest{})
	_ = Resource(UpdateStackPreviewPayloadRequest{})
	_ = Resource(ReviewRequestResponse{})
	_ = Resource(ReviewRequestResponses{})
	_ = Resource(ReviewRequestResponsePayload{})
	_ = Resource(StoreOutputRequest{})
	_ = Resource(StoreOutput{})
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

// String is a human representation of the organization.
func (org MemberOrganization) String() string {
	return fmt.Sprintf("%s (%s)", org.DisplayName, org.Name)
}

// String is a human readable list of organizations associated with a user.
func (orgs MemberOrganizations) String() string {
	str := make([]string, len(orgs))
	for i, org := range orgs {
		str[i] = org.String()
	}

	return strings.Join(str, ", ")
}

// ActiveOrgs filter the organization list by status=active organizations.
func (orgs MemberOrganizations) ActiveOrgs() MemberOrganizations {
	var res MemberOrganizations
	for _, org := range orgs {
		if org.Status == "active" {
			res = append(res, org)
		}
	}
	return res
}

// TrustedOrgs filter the organization list by status=trusted organizations.
func (orgs MemberOrganizations) TrustedOrgs() MemberOrganizations {
	var res MemberOrganizations
	for _, org := range orgs {
		if org.Status == "trusted" {
			res = append(res, org)
		}
	}
	return res
}

// InvitedOrgs filter the organization list by status=invited organizations.
func (orgs MemberOrganizations) InvitedOrgs() MemberOrganizations {
	var res MemberOrganizations
	for _, org := range orgs {
		if org.Status == "invited" {
			res = append(res, org)
		}
	}
	return res
}

// SSOInvitedOrgs filter the organization list by status=sso_invited organizations.
func (orgs MemberOrganizations) SSOInvitedOrgs() MemberOrganizations {
	var res MemberOrganizations
	for _, org := range orgs {
		if org.Status == "sso_invited" {
			res = append(res, org)
		}
	}
	return res
}

// LookupByName lookup an organization by name in the org list.
func (orgs MemberOrganizations) LookupByName(name string) (org MemberOrganization, found bool) {
	for _, org := range orgs {
		if org.Name == name {
			return org, true
		}
	}
	return MemberOrganization{}, false
}

// Validate the well-known payload.
func (wk WellKnown) Validate() error {
	if wk.RequiredVersion == "" {
		return errors.E(`missing "required_version" field`)
	}
	return nil
}

// Validate the SSO details.
func (sso SingleSignOnDetailResponse) Validate() error {
	if sso.EnterpriseOrgID == "" {
		return errors.E(`missing "enterprise_org_id" field`)
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
	return validateResourceList(ds...)
}

// Validate a drift with stack.
func (d DriftWithStack) Validate() error {
	if err := d.Stack.Validate(); err != nil {
		return err
	}
	if err := d.Drift.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate a list of drifts.
func (ds DriftsWithStacks) Validate() error {
	return validateResourceList(ds...)
}

// Validate the drift request payload.
func (d DriftCheckRunStartPayloadRequest) Validate() error {
	if err := d.Stack.Validate(); err != nil {
		return err
	}
	if d.Metadata != nil {
		err := d.Metadata.Validate()
		if err != nil {
			return err
		}
	}
	if len(d.Command) == 0 {
		return errors.E(`field "command" is required"`)
	}

	return nil
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

// Validate the UpdateStackPreviewPayloadRequest object.
func (r UpdateStackPreviewPayloadRequest) Validate() error {
	errs := errors.L()
	if r.Status == "" {
		errs.Append(errors.E("status is required"))
	}

	if r.ChangesetDetails != nil {
		if err := r.ChangesetDetails.Validate(); err != nil {
			errs.Append(err)
		}
	}
	return errs.AsError()
}

// Validate the ResponsePreviewStacks object.
func (s ResponsePreviewStacks) Validate() error {
	errs := errors.L()

	for i, stack := range s {
		if stack.MetaID == "" {
			errs.Append(errors.E(`missing "meta_id" field for stack[%d]`, i))
		}
		if stack.StackPreviewID == "" {
			errs.Append(errors.E(`missing "stack_preview_id" field for stack[%d]`, i))
		}
	}

	return errs.AsError()
}

// Validate the PreviewStacks object.
func (s PreviewStacks) Validate() error {
	errs := errors.L()
	for i, stack := range s {
		if stack.PreviewStatus == "" {
			errs.Append(errors.E(`missing "preview_status" field for stack[%d]`, i))
		}
		if stack.Cmd == nil {
			errs.Append(errors.E(`missing "cmd" field for stack[%d]`, i))
		}
		if err := stack.Validate(); err != nil {
			errs.Append(errors.E(err, "invalid attributes for stack[%d]", i))
		}
	}

	return errs.AsError()
}

// Validate the CreatePreviewPayloadRequest object.
func (r CreatePreviewPayloadRequest) Validate() error {
	errs := errors.L()
	if r.Technology == "" {
		errs.Append(errors.E(`missing "technology" field`))
	}
	if r.TechnologyLayer == "" {
		errs.Append(errors.E(`missing "technology_layer" field`))
	}
	if r.PushedAt == 0 {
		errs.Append(errors.E(`missing "pushed_at" field`))
	}
	if r.CommitSHA == "" {
		errs.Append(errors.E(`missing "commit_sha" field`))
	}
	if r.Stacks == nil {
		errs.Append(errors.E(`missing "stacks" field`))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}
	if r.ReviewRequest == nil {
		errs.Append(errors.E(`missing "review_request" field`))
	} else {
		if err := r.ReviewRequest.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

// Validate validates the CreatePreviewResponse payload
func (r CreatePreviewResponse) Validate() error {
	errs := errors.L()

	if r.PreviewID == "" {
		errs.Append(errors.E(`missing "preview_id" field`))
	}

	if r.Stacks == nil {
		errs.Append(errors.E(`missing "stacks" field`))
	} else {
		if err := r.Stacks.Validate(); err != nil {
			errs.Append(err)
		}
	}

	return errs.AsError()
}

// Validate validates the DriftCheckRunStartResponse payload
func (r DriftCheckRunStartResponse) Validate() error {
	if r.DriftUUID == "" {
		return errors.E(`missing "uuid" field`)
	}
	return nil
}

// Validate validates the UpdateDriftPayloadRequest payload
func (r UpdateDriftPayloadRequest) Validate() error {
	if err := r.Status.Validate(); err != nil {
		return err
	}
	if r.Changeset != nil {
		return r.Changeset.Validate()
	}
	if r.UpdatedAt.IsZero() {
		return errors.E(`missing "updated_at" field`)
	}
	return nil
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

// NoStatusFilters returns a StatusFilters with no filter.
func NoStatusFilters() StatusFilters {
	return StatusFilters{
		StackStatus:      stack.NoFilter,
		DeploymentStatus: deployment.NoFilter,
		DriftStatus:      drift.NoFilter,
	}
}

// HasFilter tells if StackFilter has any filter set.
func (f StatusFilters) HasFilter() bool {
	return f.StackStatus != stack.NoFilter || f.DeploymentStatus != deployment.NoFilter || f.DriftStatus != drift.NoFilter
}

// Validate the StoreOutputRequest object.
func (s StoreOutputRequest) Validate() error {
	if s.Key.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	if s.Key.StackMetaID == "" {
		return errors.E(`missing "stack_meta_id" field`)
	}
	if s.Key.Name == "" {
		return errors.E(`missing "name" field`)
	}
	return nil
}

// Validate the StoreOutputResponse object.
func (s StoreOutput) Validate() error {
	if s.ID == "" {
		return errors.E(`missing "id" field`)
	}
	if s.Key.OrgUUID == "" {
		return errors.E(`missing "org_uuid" field`)
	}
	if s.Key.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	if s.Key.StackMetaID == "" {
		return errors.E(`missing "stack_meta_id" field`)
	}
	if s.Key.Target == "" {
		return errors.E(`missing "target" field`)
	}

	// Value can be an empty string.

	if s.CreatedAt.IsZero() {
		return errors.E(`missing "created_at" field`)
	}
	if s.UpdatedAt.IsZero() {
		return errors.E(`missing "updated_at" field`)
	}
	return nil
}
