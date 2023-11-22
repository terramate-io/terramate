// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudstore

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
)

type (
	// Data is the in-memory data store.
	// It has public fields but they *SHALL NOT* be directly manipulated
	// unless for the case of initialiting the data.
	Data struct {
		mu    sync.RWMutex
		Orgs  map[string]Org        `json:"orgs"`
		Users map[string]cloud.User `json:"users"`
	}
	// Org is the organization model.
	Org struct {
		UUID        cloud.UUID `json:"uuid"`
		Name        string     `json:"name"`
		DisplayName string     `json:"display_name"`
		Domain      string     `json:"domain"`
		Status      string     `json:"status"`

		Members     []Member                   `json:"members"`
		Stacks      []Stack                    `json:"stacks"`
		Deployments map[cloud.UUID]*Deployment `json:"deployments"`
		Drifts      []*Drift                   `json:"drifts"`
	}
	// Member represents the organization member.
	Member struct {
		UserUUID cloud.UUID `json:"user_uuid"`
		Role     string     `json:"role"`
		Status   string     `json:"status"`
		MemberID int        // implicit from the members list position index.
		Org      *Org       // back-pointer set while retrieving memberships.
	}
	// Stack is the stack representation.
	Stack struct {
		cloud.Stack

		State StackState `json:"state"`
	}
	// StackState represents the state of the stack.
	StackState struct {
		Status           stack.Status      `json:"status"`
		DeploymentStatus deployment.Status `json:"deployment_status"`
		DriftStatus      drift.Status      `json:"drift_status"`
		CreatedAt        *time.Time        `json:"created_at,omitempty"`
		UpdatedAt        *time.Time        `json:"updated_at,omitempty"`
		SeenAt           *time.Time        `json:"seen_at,omitempty"`
	}
	// Deployment model.
	Deployment struct {
		UUID          cloud.UUID                     `json:"uuid"`
		Stacks        []int                          `json:"stacks"`
		Workdir       string                         `json:"workdir"`
		StackCommands map[string]string              `json:"stack_commands"`
		DeploymentURL string                         `json:"deployment_url,omitempty"`
		Status        deployment.Status              `json:"status"`
		Metadata      *cloud.DeploymentMetadata      `json:"metadata"`
		ReviewRequest *cloud.DeploymentReviewRequest `json:"review_request"`
		State         DeploymentState                `json:"state"`
	}
	// DeploymentState is the state of a deployment.
	DeploymentState struct {
		StackStatus       map[int]deployment.Status    `json:"stacks_status"`
		StackStatusEvents map[int][]deployment.Status  `json:"stacks_events"`
		StackLogs         map[int]cloud.DeploymentLogs `json:"stacks_logs"`
	}
	// Drift model.
	Drift struct {
		UUID       string                    `json:"uuid"`
		Status     stack.Status              `json:"status"`
		Details    *cloud.DriftDetails       `json:"details"`
		Metadata   *cloud.DeploymentMetadata `json:"metadata"`
		Command    []string                  `json:"command"`
		StartedAt  *time.Time                `json:"started_at,omitempty"`
		FinishedAt *time.Time                `json:"finished_at,omitempty"`
	}
)

const (
	// ErrAlreadyExists is the error for the case where the record already exists.
	ErrAlreadyExists errors.Kind = "record already exists"
	// ErrNotExists is the error when the record does not exists.
	ErrNotExists errors.Kind = "record does not exist"
)

// LoadDatastore loads the data store from a JSON file.
func LoadDatastore(fpath string) (*Data, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, errors.E(err, "failed to read testserver data")
	}
	var dstore Data
	err = json.Unmarshal(data, &dstore)
	if err != nil {
		return nil, errors.E(err, "unmarshaling data store from file %s", fpath)
	}
	return &dstore, nil
}

// MarshalJSON implements the [json.Marshaler] interface to the data store.
// It's required to avoid data races if store is concurrently accessed.
func (d *Data) MarshalJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var ret struct {
		Orgs  map[string]Org        `json:"orgs"`
		Users map[string]cloud.User `json:"users"`
	}
	ret.Orgs = d.Orgs
	ret.Users = d.Users
	return json.Marshal(ret)
}

// GetUser retrieves the user by email from the store.
func (d *Data) GetUser(email string) (cloud.User, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, user := range d.Users {
		if user.Email == email {
			return user, true
		}
	}
	return cloud.User{}, false
}

// MustGetUser retrieves the user by email or panics.
func (d *Data) MustGetUser(email string) cloud.User {
	u, ok := d.GetUser(email)
	if !ok {
		panic(errors.E("user with email %s not found", email))
	}
	return u
}

// OrgByName returns an organization with the provided name.
func (d *Data) OrgByName(name string) (Org, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	org, ok := d.Orgs[name]
	return org, ok
}

// MustOrgByName must return an organization with the provided name otherwise it panics.
func (d *Data) MustOrgByName(name string) Org {
	org, found := d.OrgByName(name)
	if !found {
		panic(errors.E("org %s not found", name))
	}
	return org
}

// GetOrg retrieves the organization with the provided uuid.
func (d *Data) GetOrg(uuid cloud.UUID) (Org, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, org := range d.Orgs {
		if org.UUID == uuid {
			return org, true
		}
	}
	return Org{}, false
}

// UpsertOrg inserts or updates the provided organization.
func (d *Data) UpsertOrg(org Org) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Orgs[org.Name] = org
}

// GetMemberships returns the organizations that user is member of.
func (d *Data) GetMemberships(user cloud.User) []Member {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var memberships []Member
outer:
	for _, org := range d.Orgs {
		for _, member := range org.Members {
			if member.UserUUID == user.UUID {
				orgCopy := org
				member.Org = &orgCopy
				memberships = append(memberships, member)
				continue outer
			}
		}
	}
	sort.Slice(memberships, func(i, j int) bool {
		return memberships[i].Org.Name < memberships[j].Org.Name
	})
	return memberships
}

// GetStackByMetaID returns the given stack.
func (d *Data) GetStackByMetaID(org Org, id string) (*Stack, int, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, st := range org.Stacks {
		if st.Stack.MetaID == id {
			return &st, i, true
		}
	}
	return nil, 0, false
}

// GetStack by id.
func (d *Data) GetStack(org Org, id int) (Stack, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if id < 0 || id >= len(org.Stacks) {
		return Stack{}, false
	}
	return org.Stacks[id], true
}

// UpsertStack inserts or updates the given stack.
func (d *Data) UpsertStack(orguuid cloud.UUID, st Stack) (int, error) {
	org, found := d.GetOrg(orguuid)
	if !found {
		return 0, errors.E(ErrNotExists, "org uuid %s", orguuid)
	}

	t := time.Now().UTC()
	if st.State.DriftStatus == 0 {
		st.State.DriftStatus = drift.Unknown
	}
	if st.State.DeploymentStatus == 0 {
		st.State.DeploymentStatus = deployment.OK
	}
	st.State.CreatedAt = &t
	st.State.UpdatedAt = &t
	st.State.SeenAt = &t

	_, id, found := d.GetStackByMetaID(org, st.Stack.MetaID)

	d.mu.Lock()
	defer d.mu.Unlock()

	if found {
		org.Stacks[id] = st
		d.Orgs[org.Name] = org
		return id, nil
	}
	org = d.Orgs[org.Name]
	org.Stacks = append(org.Stacks, st)
	d.Orgs[org.Name] = org
	return len(org.Stacks) - 1, nil
}

// GetDeployment returns the given deployment.
func (d *Data) GetDeployment(org *Org, id cloud.UUID) (*Deployment, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, deployment := range org.Deployments {
		if deployment.UUID == id {
			return deployment, true
		}
	}
	return nil, false
}

// InsertDeployment inserts the given deployment in the data store.
func (d *Data) InsertDeployment(orgID cloud.UUID, deploy Deployment) error {
	org, found := d.GetOrg(orgID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := org.Deployments[deploy.UUID]; exists {
		return errors.E(ErrAlreadyExists, "deployment uuid %s", string(deploy.UUID))
	}
	copyDeployment := deploy
	copyDeployment.State.StackStatus = make(map[int]deployment.Status)
	copyDeployment.State.StackLogs = make(map[int]cloud.DeploymentLogs)
	copyDeployment.State.StackStatusEvents = make(map[int][]deployment.Status)
	for _, stackID := range deploy.Stacks {
		copyDeployment.State.StackStatusEvents[stackID] = append(copyDeployment.State.StackStatusEvents[stackID], deployment.Pending)
	}
	if org.Deployments == nil {
		org.Deployments = make(map[cloud.UUID]*Deployment)
	}
	org.Deployments[deploy.UUID] = &copyDeployment
	d.Orgs[org.Name] = org
	return nil
}

// SetDeploymentStatus sets the given deployment stack to the given status.
func (d *Data) SetDeploymentStatus(org Org, deploymentID cloud.UUID, stackID int, status deployment.Status) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	deployment, exists := org.Deployments[deploymentID]
	if !exists {
		return errors.E(ErrNotExists, "deployment uuid %s", deploymentID)
	}
	deployment.State.StackStatus[stackID] = status
	deployment.State.StackStatusEvents[stackID] = append(deployment.State.StackStatusEvents[stackID], status)
	return nil
}

// GetDeploymentEvents returns the events of the given deployment.
func (d *Data) GetDeploymentEvents(orgID, deploymentID cloud.UUID) (map[string][]deployment.Status, error) {
	org, found := d.GetOrg(orgID)
	if !found {
		return nil, errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	deploy, exists := org.Deployments[deploymentID]
	if !exists {
		return nil, errors.E(ErrNotExists, "deployment uuid %s", deploymentID)
	}
	eventsPerStack := map[string][]deployment.Status{}
	for stackID, events := range deploy.State.StackStatusEvents {
		eventsPerStack[org.Stacks[stackID].MetaID] = events
	}
	return eventsPerStack, nil
}

// InsertDeploymentLogs inserts logs for the given deployment.
func (d *Data) InsertDeploymentLogs(
	orgID cloud.UUID,
	stackMetaID string,
	deploymentID cloud.UUID,
	logs cloud.DeploymentLogs,
) error {
	org, found := d.GetOrg(orgID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	_, stackID, found := d.GetStackByMetaID(org, stackMetaID)
	if !found {
		return errors.E(ErrNotExists, "stack id %s", stackMetaID)
	}
	deployment, found := d.GetDeployment(&org, deploymentID)
	if !found {
		return errors.E(ErrNotExists, "deployment uuid %s", deploymentID)
	}
	d.mu.Lock()
	deployment.State.StackLogs[stackID] = append(deployment.State.StackLogs[stackID], logs...)
	d.mu.Unlock()
	return nil
}

// GetDeploymentLogs returns the logs of the given deployment.
func (d *Data) GetDeploymentLogs(orgID cloud.UUID, stackMetaID string, deploymentID cloud.UUID, fromLine int) (cloud.DeploymentLogs, error) {
	org, found := d.GetOrg(orgID)
	if !found {
		return nil, errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	_, stackID, found := d.GetStackByMetaID(org, stackMetaID)
	if !found {
		return nil, errors.E(ErrNotExists, "stack meta_id %s", stackMetaID)
	}
	deploy, found := d.GetDeployment(&org, deploymentID)
	if !found {
		return nil, errors.E(ErrNotExists, "deployment uuid %s", deploymentID)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	logs := deploy.State.StackLogs[stackID]
	if fromLine < 0 || fromLine >= len(logs) {
		return nil, errors.E("logs range out of bounds")
	}
	return logs[fromLine:], nil
}
