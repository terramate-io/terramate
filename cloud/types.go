// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"

	"github.com/terramate-io/terramate/errors"
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

	// Stack represents a stack in the Terramate Cloud.
	Stack struct {
		ID              int      `json:"stack_id"`
		Repository      string   `json:"repository"`
		Path            string   `json:"path"`
		MetaID          string   `json:"meta_id"`
		MetaName        string   `json:"meta_name"`
		MetaDescription string   `json:"meta_description"`
		MetaTags        []string `json:"meta_tags"`
		Status          Status   `json:"status"`

		// readonly fields
		CreatedAt int `json:"created_at"`
		UpdatedAt int `json:"updated_at"`
		SeenAt    int `json:"seen_at"`
	}

	// DeploymentStackRequest represents the stack object of the request payload
	// type for the creation of stack deployments.
	DeploymentStackRequest struct {
		CommitSHA       string   `json:"commit_sha,omitempty"`
		Repository      string   `json:"repository"`
		Path            string   `json:"path"`
		MetaID          string   `json:"meta_id"`
		MetaName        string   `json:"meta_name"`
		MetaDescription string   `json:"meta_description"`
		MetaTags        []string `json:"meta_tags"`
		DeploymentURL   string   `json:"deployment_url,omitempty"`
		RequestURL      string   `json:"request_url,omitempty"`
		Status          Status   `json:"status"`
		Command         string   `json:"cmd"`
	}

	// DeploymentStackResponse represents the deployment creation response item.
	DeploymentStackResponse struct {
		StackID     int    `json:"stack_id"`
		StackMetaID string `json:"meta_id"`
		Status      Status `json:"status"`
	}

	// DeploymentStacksResponse represents the list of DeploymentStackResponse.
	DeploymentStacksResponse []DeploymentStackResponse

	// DeploymentStackRequests is a list of DeploymentStacksRequest.
	DeploymentStackRequests []DeploymentStackRequest

	// DeploymentStacksPayloadRequest is the request payload for the creation of stack deployments.
	DeploymentStacksPayloadRequest struct {
		Stacks DeploymentStackRequests `json:"stacks"`
	}

	// UpdateDeploymentStack is the request payload item for updating the deployment status.
	UpdateDeploymentStack struct {
		StackID int    `json:"stack_id"`
		Status  Status `json:"status"`
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
	_ = Resource(Stack{})
	_ = Resource(DeploymentStackRequest{})
	_ = Resource(DeploymentStackRequests{})
	_ = Resource(DeploymentStacksPayloadRequest{})
	_ = Resource(DeploymentStackResponse{})
	_ = Resource(DeploymentStacksResponse{})
	_ = Resource(UpdateDeploymentStack{})
	_ = Resource(UpdateDeploymentStacks{})
	_ = Resource(empty(""))
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
			write(org.DisplayName)
			if i+1 < len(orgs) {
				write(", ")
			}
		}
	}
	return out.String()
}

// Validate if the user has the Terramate CLI required fields.
func (u User) Validate() error {
	if u.DisplayName == "" {
		return errors.E(`missing "display_name" field.`)
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
func (stack Stack) Validate() error {
	return nil
}

// Validate the deployment stack request.
func (d DeploymentStackRequest) Validate() error {
	if d.Repository == "" {
		return errors.E(`missing "repository" field`)
	}
	if d.Path == "" {
		return errors.E(`missing "path" field`)
	}
	if d.MetaID == "" {
		return errors.E(`missing "meta_id" field`)
	}
	if d.Command == "" {
		return errors.E(`missing "cmd" field`)
	}
	return nil
}

// Validate the list of deployment stack requests.
func (d DeploymentStackRequests) Validate() error { return validateResourceList(d...) }

// Validate the deployment stack payload.
func (d DeploymentStacksPayloadRequest) Validate() error { return validateResourceList(d.Stacks) }

// Validate the deployment stack response.
func (d DeploymentStackResponse) Validate() error {
	return d.Status.Validate()
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

type empty string

func (s empty) Validate() error {
	if s == "" {
		return nil
	}
	return errors.E("unexpected non-empty string")
}
