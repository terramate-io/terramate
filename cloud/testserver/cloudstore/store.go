// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudstore

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/drift"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/cloud/stack"
	"github.com/terramate-io/terramate/errors"
)

type (
	// Data is the in-memory data store.
	// It has public fields but they *SHALL NOT* be directly manipulated
	// unless for the case of initialiting the data.
	Data struct {
		mu                    sync.RWMutex
		Orgs                  map[string]Org        `json:"orgs"`
		Users                 map[string]cloud.User `json:"users"`
		WellKnown             *cloud.WellKnown      `json:"well_known"`
		previewIDAutoInc      int
		stackPreviewIDAutoInc int
		Github                struct {
			GetPullRequestResponse json.RawMessage `json:"get_pull_request_response"`
			GetCommitResponse      json.RawMessage `json:"get_commit_response"`
		} `json:"github"`
	}
	// Org is the organization model.
	Org struct {
		UUID        cloud.UUID `json:"uuid"`
		Name        string     `json:"name"`
		DisplayName string     `json:"display_name"`
		Domain      string     `json:"domain"`
		Status      string     `json:"status"`

		Members        []Member                   `json:"members"`
		Stacks         []Stack                    `json:"stacks"`
		Deployments    map[cloud.UUID]*Deployment `json:"deployments"`
		Drifts         []Drift                    `json:"drifts"`
		Previews       []Preview                  `json:"previews"`
		ReviewRequests []cloud.ReviewRequest      `json:"review_requests"`
	}
	//Preview is the preview model.
	Preview struct {
		PreviewID string `json:"preview_id"`

		UpdatedAt       int64                     `json:"updated_at"`
		PushedAt        int64                     `json:"pushed_at"`
		CommitSHA       string                    `json:"commit_sha"`
		Technology      string                    `json:"technology"`
		TechnologyLayer string                    `json:"technology_layer"`
		ReviewRequest   *cloud.ReviewRequest      `json:"review_request,omitempty"`
		Metadata        *cloud.DeploymentMetadata `json:"metadata,omitempty"`
		StackPreviews   []*StackPreview           `json:"stack_previews"`
	}

	// StackPreview is the stack preview model.
	StackPreview struct {
		Stack

		ID               string                  `json:"stack_preview_id"`
		Status           preview.StackStatus     `json:"status"`
		Cmd              []string                `json:"cmd,omitempty"`
		ChangesetDetails *cloud.ChangesetDetails `json:"changeset_details,omitempty"`
		Logs             cloud.CommandLogs       `json:"logs,omitempty"`
	}

	// Member represents the organization member.
	Member struct {
		UserUUID cloud.UUID `json:"user_uuid"`
		Role     string     `json:"role"`
		Status   string     `json:"status"`
		MemberID int64      // implicit from the members list position index.
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
		UUID          cloud.UUID                `json:"uuid"`
		Stacks        []int64                   `json:"stacks"`
		Workdir       string                    `json:"workdir"`
		StackCommands map[string]string         `json:"stack_commands"`
		DeploymentURL string                    `json:"deployment_url,omitempty"`
		Status        deployment.Status         `json:"status"`
		Metadata      *cloud.DeploymentMetadata `json:"metadata"`
		ReviewRequest *cloud.ReviewRequest      `json:"review_request"`
		State         DeploymentState           `json:"state"`
	}
	// DeploymentState is the state of a deployment.
	DeploymentState struct {
		StackStatus       map[int64]deployment.Status   `json:"stacks_status"`
		StackStatusEvents map[int64][]deployment.Status `json:"stacks_events"`
		StackLogs         map[int64]cloud.CommandLogs   `json:"stacks_logs"`
	}
	// Drift model.
	Drift struct {
		ID          int64                     `json:"id"`
		StackMetaID string                    `json:"stack_meta_id"`
		Status      drift.Status              `json:"status"`
		Details     *cloud.ChangesetDetails   `json:"details"`
		Metadata    *cloud.DeploymentMetadata `json:"metadata"`
		Command     []string                  `json:"command"`
		StartedAt   *time.Time                `json:"started_at,omitempty"`
		FinishedAt  *time.Time                `json:"finished_at,omitempty"`
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

// GetWellKnown gets the defined well-known.
func (d *Data) GetWellKnown() *cloud.WellKnown {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.WellKnown
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
func (d *Data) GetStackByMetaID(org Org, id string) (Stack, int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, st := range org.Stacks {
		if st.Stack.MetaID == id {
			return st, int64(i), true
		}
	}
	return Stack{}, 0, false
}

// GetStack by id.
func (d *Data) GetStack(org Org, id int64) (Stack, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if id < 0 || id >= int64(len(org.Stacks)) {
		return Stack{}, false
	}
	return org.Stacks[id], true
}

// UpsertStack inserts or updates the given stack.
func (d *Data) UpsertStack(orguuid cloud.UUID, st Stack) (int64, error) {
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
	if st.State.Status == 0 {
		st.State.Status = stack.OK
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
	return int64(len(org.Stacks) - 1), nil
}

// AppendPreviewLogs appends logs to the given stack preview.
func (d *Data) AppendPreviewLogs(org Org, stackPreviewID string, logs cloud.CommandLogs) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range org.Previews {
		for _, sp := range p.StackPreviews {
			if sp.ID == stackPreviewID {
				sp.Logs = append(sp.Logs, logs...)
				return nil
			}
		}
	}
	return errors.E(ErrNotExists, "stack preview id %s", stackPreviewID)
}

// UpsertPreview inserts or updates the given preview.
func (d *Data) UpsertPreview(orguuid cloud.UUID, p Preview) (string, error) {
	org, found := d.GetOrg(orguuid)
	if !found {
		return "", errors.E(ErrNotExists, "org uuid %s", orguuid)
	}

	d.upsertReviewRequest(org, p.ReviewRequest)
	for _, sp := range p.StackPreviews {
		_, err := d.UpsertStack(orguuid, sp.Stack)
		if err != nil {
			return "", errors.E(err, "failed to upsert stack")
		}
	}

	_, id, found := d.getPreview(org, p.ReviewRequest.Number, p.UpdatedAt)
	d.mu.Lock()
	defer d.mu.Unlock()

	p.PreviewID = d.newPreviewID()

	if found {
		org.Previews[id] = p
		d.Orgs[org.Name] = org

		for _, sp := range p.StackPreviews {
			_, err := d.UpsertStackPreview(org, p.PreviewID, sp)
			if err != nil {
				return "", errors.E(err, "failed to upsert stack preview")
			}
		}
		return p.PreviewID, nil
	}

	org = d.Orgs[org.Name]
	org.Previews = append(org.Previews, p)
	d.Orgs[org.Name] = org

	return p.PreviewID, nil
}

// GetPreviewByID returns the preview associated with previewID.
func (d *Data) GetPreviewByID(org Org, previewID string) (Preview, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i := range org.Previews {
		if org.Previews[i].PreviewID == previewID {
			return org.Previews[i], true
		}
	}
	return Preview{}, false
}

// UpsertStackPreview inserts or updates the given stack preview.
func (d *Data) UpsertStackPreview(org Org, previewID string, sp *StackPreview) (string, error) {
	_, pIndex, found := d.getPreviewByID(org, previewID)
	d.mu.Lock()
	defer d.mu.Unlock()

	if found {
		stackPreviews := org.Previews[pIndex].StackPreviews
		_, spIndex, spFound := d.getStackPreviewByMetaID(sp.Stack.MetaID, stackPreviews)
		if spFound {
			stackPreviews[spIndex] = sp
			d.Orgs[org.Name] = org
			return stackPreviews[spIndex].ID, nil
		}

		sp.ID = d.newStackPreviewID()
		org.Previews[pIndex].StackPreviews = append(org.Previews[pIndex].StackPreviews, sp)
		d.Orgs[org.Name] = org
		return sp.ID, nil
	}

	return "", errors.E(ErrNotExists, "preview id %s", previewID)
}

// UpdateStackPreview updates the given stack preview.
func (d *Data) UpdateStackPreview(org Org, stackPreviewID string, status string, changeset *cloud.ChangesetDetails) (*StackPreview, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range org.Previews {
		for _, sp := range p.StackPreviews {
			if sp.ID == stackPreviewID {
				sp.Status = preview.StackStatus(status)
				if changeset != nil {
					sp.ChangesetDetails = changeset
				}
				return sp, true
			}
		}
	}
	return nil, false
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

// GetStackDrifts returns the drifts of the provided stack.
func (d *Data) GetStackDrifts(orguuid cloud.UUID, stackID int64) ([]Drift, error) {
	org, found := d.GetOrg(orguuid)
	if !found {
		return nil, errors.E(ErrNotExists, "org uuid %s", orguuid)
	}
	st, found := d.GetStack(org, stackID)
	if !found {
		return nil, errors.E(ErrNotExists, "stack id %d", stackID)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	var drifts []Drift
	for i, drift := range org.Drifts {
		drift.ID = int64(i) // lazy set, then can be unset in HCL
		if drift.StackMetaID == st.MetaID {
			drifts = append(drifts, drift)
		}
	}
	return drifts, nil
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

	deploy.State.StackStatus = make(map[int64]deployment.Status)
	deploy.State.StackLogs = make(map[int64]cloud.CommandLogs)
	deploy.State.StackStatusEvents = make(map[int64][]deployment.Status)
	for _, stackID := range deploy.Stacks {
		deploy.State.StackStatusEvents[stackID] = append(deploy.State.StackStatusEvents[stackID], deployment.Pending)
	}
	if org.Deployments == nil {
		org.Deployments = make(map[cloud.UUID]*Deployment)
	}
	org.Deployments[deploy.UUID] = &deploy
	d.Orgs[org.Name] = org
	return nil
}

// FindDeploymentForCommit returns the deployment for the given commit.
func (d *Data) FindDeploymentForCommit(orgID cloud.UUID, commitSHA string) (*Deployment, bool) {
	org, found := d.GetOrg(orgID)
	if !found {
		return nil, false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, deployment := range org.Deployments {
		if deployment.Metadata.GitCommitSHA == commitSHA {
			return deployment, true
		}
	}
	return nil, false
}

// InsertDrift inserts a new drift into the store for the provided org.
func (d *Data) InsertDrift(orgID cloud.UUID, drift Drift) (int, error) {
	org, found := d.GetOrg(orgID)
	if !found {
		return 0, errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	org.Drifts = append(org.Drifts, drift)
	d.Orgs[org.Name] = org
	return len(org.Drifts) - 1, nil
}

// SetDeploymentStatus sets the given deployment stack to the given status.
func (d *Data) SetDeploymentStatus(org Org, deploymentID cloud.UUID, stackID int64, status deployment.Status) error {
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
	logs cloud.CommandLogs,
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
func (d *Data) GetDeploymentLogs(orgID cloud.UUID, stackMetaID string, deploymentID cloud.UUID, fromLine int) (cloud.CommandLogs, error) {
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

// NewState creates a new valid state.
func NewState() StackState {
	return StackState{
		Status:           stack.OK,
		DeploymentStatus: deployment.OK,
		DriftStatus:      drift.OK,
	}
}

// GetGithubCommitResponse returns the github commit response.
func (d *Data) GetGithubCommitResponse() json.RawMessage {
	return d.Github.GetCommitResponse
}

// GetGithubPullRequestResponse returns the github pull request response.
func (d *Data) GetGithubPullRequestResponse() json.RawMessage {
	return d.Github.GetPullRequestResponse
}

func (d *Data) getStackPreviewByMetaID(spMetaID string, stackPreviews []*StackPreview) (*StackPreview, int64, bool) {
	for i := range stackPreviews {
		if stackPreviews[i].Stack.MetaID == spMetaID {
			return stackPreviews[i], int64(i), true
		}
	}
	return nil, 0, false
}

func (d *Data) getPreviewByID(org Org, id string) (Preview, int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, p := range org.Previews {
		if p.PreviewID == id {
			return p, int64(i), true
		}
	}
	return Preview{}, 0, false
}

func (d *Data) newPreviewID() string {
	d.previewIDAutoInc++
	return strconv.Itoa(d.previewIDAutoInc)
}

func (d *Data) newStackPreviewID() string {
	d.stackPreviewIDAutoInc++
	return strconv.Itoa(d.stackPreviewIDAutoInc)
}

func (d *Data) upsertReviewRequest(org Org, newRR *cloud.ReviewRequest) {
	rrIndex, rrFound := d.getReviewRequest(org, newRR.Number)
	if !rrFound {
		org = d.Orgs[org.Name]
		org.ReviewRequests = append(org.ReviewRequests, *newRR)
		d.Orgs[org.Name] = org
		return
	}

	org = d.Orgs[org.Name]
	org.ReviewRequests[rrIndex] = *newRR
	d.Orgs[org.Name] = org
}

func (d *Data) getReviewRequest(org Org, rrNumber int) (int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for i, rr := range d.Orgs[org.Name].ReviewRequests {
		if rr.Number == rrNumber {
			return int64(i), true
		}
	}
	return 0, false
}

func (d *Data) getPreview(org Org, rrNumber int, updatedAt int64) (Preview, int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, p := range org.Previews {
		if org.Previews[i].ReviewRequest.Number == rrNumber && org.Previews[i].UpdatedAt == updatedAt {
			return p, int64(i), true
		}
	}
	return Preview{}, 0, false
}
