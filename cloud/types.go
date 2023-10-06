// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"
	"strings"
	"time"

	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
)

type (
	// Resource is the interface used to represent resource entities.
	Resource interface {
		Validate() error
	}

	// MemberOrganizations is a list of organizations associated with the member.
	MemberOrganizations []MemberOrganization

	// User represents the signed in user information.
	User struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		JobTitle    string `json:"job_title"`
		IDPUserID   string `json:"idp_user_id"`
	}

	// MemberOrganization represents the organization associated with the member.
	MemberOrganization struct {
		MemberID    int    `json:"member_id,omitempty"`
		Name        string `json:"org_name"`
		DisplayName string `json:"org_display_name"`
		Domain      string `json:"org_domain"`
		UUID        string `json:"org_uuid"`
		Role        string `json:"role,omitempty"`
		Status      string `json:"status"`
	}

	// StackResponse represents a stack in the Terramate Cloud.
	StackResponse struct {
		ID int `json:"stack_id"`
		Stack
		Status stack.Status `json:"status"`

		// readonly fields
		CreatedAt *time.Time `json:"created_at,omitempty"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
		SeenAt    *time.Time `json:"seen_at,omitempty"`
	}

	// Stack represents the stack as defined by the user HCL code.
	Stack struct {
		Repository      string   `json:"repository"`
		Path            string   `json:"path"`
		MetaID          string   `json:"meta_id"`
		MetaName        string   `json:"meta_name,omitempty"`
		MetaDescription string   `json:"meta_description,omitempty"`
		MetaTags        []string `json:"meta_tags,omitempty"`
	}

	// DriftDetails represents the details of a drift.
	DriftDetails struct {
		Provisioner    string `json:"provisioner"`
		ChangesetASCII string `json:"changeset_ascii,omitempty"`
		ChangesetJSON  string `json:"changeset_json,omitempty"`
	}

	// StacksResponse represents the stacks object response.
	StacksResponse struct {
		Stacks []StackResponse `json:"stacks"`
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
		StackID     int               `json:"stack_id"`
		StackMetaID string            `json:"meta_id"`
		Status      deployment.Status `json:"status"`
	}

	// DeploymentStacksResponse represents the list of DeploymentStackResponse.
	DeploymentStacksResponse []DeploymentStackResponse

	// DeploymentStackRequests is a list of DeploymentStacksRequest.
	DeploymentStackRequests []DeploymentStackRequest

	// DeploymentStacksPayloadRequest is the request payload for the creation of stack deployments.
	DeploymentStacksPayloadRequest struct {
		ReviewRequest *DeploymentReviewRequest `json:"review_request,omitempty"`
		Stacks        DeploymentStackRequests  `json:"stacks"`
		Workdir       project.Path             `json:"workdir"`
		Metadata      *DeploymentMetadata      `json:"metadata,omitempty"`
	}

	// DriftStackPayloadRequest is the payload for the drift sync.
	DriftStackPayloadRequest struct {
		Stack    Stack               `json:"stack"`
		Status   stack.Status        `json:"drift_status"`
		Details  *DriftDetails       `json:"drift_details,omitempty"`
		Metadata *DeploymentMetadata `json:"metadata,omitempty"`
		Command  []string            `json:"command"`
	}

	// DriftStackPayloadRequests is a list of DriftStackPayloadRequest
	DriftStackPayloadRequests []DriftStackPayloadRequest

	// DeploymentMetadata stores the metadata available in the target platform.
	// For now, we only support GitHub and Github Actions Metadata.
	// It's marshaled as a flat hashmap of values.
	// Note: no sensitive information must be stored here because it could be logged.
	DeploymentMetadata struct {
		GitHeadCommitSHA string `json:"git_head_commit_sha,omitempty"`

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
	}

	// DeploymentReviewRequest is the review_request object.
	DeploymentReviewRequest struct {
		Platform    string `json:"platform"`
		Repository  string `json:"repository"`
		CommitSHA   string `json:"commit_sha"`
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}

	// UpdateDeploymentStack is the request payload item for updating the deployment status.
	UpdateDeploymentStack struct {
		StackID int               `json:"stack_id"`
		Status  deployment.Status `json:"status"`
	}

	// UpdateDeploymentStacks is the request payload for updating the deployment status.
	UpdateDeploymentStacks struct {
		Stacks []UpdateDeploymentStack `json:"stacks"`
	}
)

var (
	// compile-time checks to ensure resource entities implement the Resource iface.
	_ = Resource(User{})
	_ = Resource(MemberOrganization{})
	_ = Resource(MemberOrganizations{})
	_ = Resource(StackResponse{})
	_ = Resource(StacksResponse{})
	_ = Resource(DeploymentStackRequest{})
	_ = Resource(DeploymentStackRequests{})
	_ = Resource(DeploymentStacksPayloadRequest{})
	_ = Resource(DeploymentStackResponse{})
	_ = Resource(DeploymentStacksResponse{})
	_ = Resource(UpdateDeploymentStack{})
	_ = Resource(UpdateDeploymentStacks{})
	_ = Resource(DeploymentReviewRequest{})
	_ = Resource(DriftStackPayloadRequest{})
	_ = Resource(DriftStackPayloadRequests{})
	_ = Resource(DriftDetails{})
	_ = Resource(EmptyResponse(""))
)

// String representation of the list of organization associated with the user.
func (orgs MemberOrganizations) String() string {
	var out bytes.Buffer

	write := func(s string) {
		// only possible error is OutOfMemory which panics already
		_, _ = out.Write([]byte(s))
	}

	if len(orgs) == 0 {
		write("none")
	} else {
		for i, org := range orgs {
			write(org.Name)
			if i+1 < len(orgs) {
				write(", ")
			}
		}
	}
	return out.String()
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
func (stack StackResponse) Validate() error {
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
	return nil
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

// Validate the drift request payload.
func (d DriftStackPayloadRequest) Validate() error {
	if err := d.Stack.Validate(); err != nil {
		return err
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

// Validate the drift details.
func (ds DriftDetails) Validate() error {
	if ds.Provisioner == "" {
		return errors.E(`field "provisioner" is required`)
	}
	if ds.ChangesetASCII == "" && ds.ChangesetJSON == "" {
		return errors.E(`either "changeset_ascii" or "changeset_json" must be set`)
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
func (rr DeploymentReviewRequest) Validate() error {
	if rr.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	return nil
}

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

func validateResourceList[T Resource](resources ...T) error {
	for _, resource := range resources {
		err := resource.Validate()
		if err != nil {
			return err
		}
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
