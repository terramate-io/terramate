// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloudstore

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/terramate-io/terramate/cloud/api/deployment"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/preview"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/api/stack"
	"github.com/terramate-io/terramate/errors"
)

type (
	// Data is the in-memory data store.
	// It has public fields but they *SHALL NOT* be directly manipulated
	// unless for the case of initialiting the data.
	Data struct {
		mu                    sync.RWMutex
		Orgs                  map[string]Org            `json:"orgs"`
		Users                 map[string]resources.User `json:"users"`
		WellKnown             *resources.WellKnown      `json:"well_known"`
		previewIDAutoInc      int
		stackPreviewIDAutoInc int
		Github                struct {
			GetPullRequestResponse json.RawMessage `json:"get_pull_request_response"`
			GetCommitResponse      json.RawMessage `json:"get_commit_response"`
		} `json:"github"`
	}
	// Org is the organization model.
	Org struct {
		UUID        resources.UUID `json:"uuid"`
		Name        string         `json:"name"`
		DisplayName string         `json:"display_name"`
		Domain      string         `json:"domain"`
		Status      string         `json:"status"`

		Members        []Member                         `json:"members"`
		Stacks         []Stack                          `json:"stacks"`
		Deployments    map[resources.UUID]*Deployment   `json:"deployments"`
		Drifts         []Drift                          `json:"drifts"`
		Previews       []Preview                        `json:"previews"`
		ReviewRequests []resources.ReviewRequest        `json:"review_requests"`
		Outputs        map[string]resources.StoreOutput `json:"outputs"` // map of (encoded key) -> output'
	}

	// OutputKey is the primary key of an output.
	OutputKey struct {
		OrgUUID     resources.UUID `json:"org_uuid"`
		Repository  string         `json:"repository"`
		StackMetaID string         `json:"stack_meta_id"`
		Target      string         `json:"target"`
		Name        string         `json:"name"`
	}

	//Preview is the preview model.
	Preview struct {
		PreviewID string `json:"preview_id"`

		PushedAt        int64                         `json:"pushed_at"`
		CommitSHA       string                        `json:"commit_sha"`
		Technology      string                        `json:"technology"`
		TechnologyLayer string                        `json:"technology_layer"`
		ReviewRequest   *resources.ReviewRequest      `json:"review_request,omitempty"`
		Metadata        *resources.DeploymentMetadata `json:"metadata,omitempty"`
		StackPreviews   []*StackPreview               `json:"stack_previews"`
	}

	// StackPreview is the stack preview model.
	StackPreview struct {
		Stack

		ID               string                      `json:"stack_preview_id"`
		Status           preview.StackStatus         `json:"status"`
		Cmd              []string                    `json:"cmd,omitempty"`
		ChangesetDetails *resources.ChangesetDetails `json:"changeset_details,omitempty"`
		Logs             resources.CommandLogs       `json:"logs,omitempty"`
	}

	// Member represents the organization member.
	Member struct {
		UserUUID resources.UUID `json:"user_uuid"`
		APIKey   string         `json:"apikey"`
		Role     string         `json:"role"`
		Status   string         `json:"status"`
		MemberID int64          // implicit from the members list position index.
		Org      *Org           // back-pointer set while retrieving memberships.
	}
	// Stack is the stack representation.
	Stack struct {
		resources.Stack

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
		UUID          resources.UUID                `json:"uuid"`
		Stacks        []int64                       `json:"stacks"`
		Workdir       string                        `json:"workdir"`
		StackCommands map[string]string             `json:"stack_commands"`
		DeploymentURL string                        `json:"deployment_url,omitempty"`
		Status        deployment.Status             `json:"status"`
		Metadata      *resources.DeploymentMetadata `json:"metadata"`
		ReviewRequest *resources.ReviewRequest      `json:"review_request"`
		State         DeploymentState               `json:"state"`
	}
	// DeploymentState is the state of a deployment.
	DeploymentState struct {
		StackStatus       map[int64]deployment.Status     `json:"stacks_status"`
		StackStatusEvents map[int64][]deployment.Status   `json:"stacks_events"`
		StackLogs         map[int64]resources.CommandLogs `json:"stacks_logs"`
	}
	// Drift model.
	Drift struct {
		ID          int64                         `json:"id"`
		StackMetaID string                        `json:"stack_meta_id"`
		StackTarget string                        `json:"stack_target"`
		Status      drift.Status                  `json:"status"`
		Details     *resources.ChangesetDetails   `json:"details"`
		Metadata    *resources.DeploymentMetadata `json:"metadata"`
		Command     []string                      `json:"command"`
		StartedAt   *time.Time                    `json:"started_at,omitempty"`
		FinishedAt  *time.Time                    `json:"finished_at,omitempty"`
	}
)

const (
	// ErrAlreadyExists is the error for the case where the record already exists.
	ErrAlreadyExists errors.Kind = "record already exists"
	// ErrNotExists is the error when the record does not exists.
	ErrNotExists errors.Kind = "record does not exist"
)

// LoadDatastore loads the data store from a JSON file.
func LoadDatastore(fpath string) (store *Data, defaultOrg string, err error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, "", errors.E(err, "failed to read testserver data")
	}
	type serverData struct {
		DefaultTestOrg string `json:"default_test_org"`
		Data
	}
	var dstore serverData
	err = json.Unmarshal(data, &dstore)
	if err != nil {
		return nil, "", errors.E(err, "unmarshaling data store from file %s", fpath)
	}
	return &dstore.Data, dstore.DefaultTestOrg, nil
}

// MarshalJSON implements the [json.Marshaler] interface to the data store.
// It's required to avoid data races if store is concurrently accessed.
func (d *Data) MarshalJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var ret struct {
		Orgs  map[string]Org            `json:"orgs"`
		Users map[string]resources.User `json:"users"`
	}
	ret.Orgs = d.Orgs
	ret.Users = d.Users
	return json.Marshal(ret)
}

// GetWellKnown gets the defined well-known.
func (d *Data) GetWellKnown() *resources.WellKnown {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.WellKnown
}

// GetUser retrieves the user by email from the store.
func (d *Data) GetUser(email string) (resources.User, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, user := range d.Users {
		if user.Email == email {
			return user, true
		}
	}
	return resources.User{}, false
}

// MustGetUser retrieves the user by email or panics.
func (d *Data) MustGetUser(email string) resources.User {
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
func (d *Data) GetOrg(uuid resources.UUID) (Org, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, org := range d.Orgs {
		if org.UUID == uuid {
			return org.Clone(), true
		}
	}
	return Org{}, false
}

// UpsertOrg inserts or updates the provided organization.
func (d *Data) UpsertOrg(org Org) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Orgs == nil {
		d.Orgs = make(map[string]Org)
	}
	d.Orgs[org.Name] = org
}

// GetMemberships returns the organizations that user is member of.
func (d *Data) GetMemberships(user resources.User) []Member {
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

// GetMembershipsForKey returns the memberships for the given key.
func (d *Data) GetMembershipsForKey(key string) []Member {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var memberships []Member
outer:
	for _, org := range d.Orgs {
		for _, member := range org.Members {
			if member.APIKey == key {
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
func (d *Data) GetStackByMetaID(org Org, id string, target string) (Stack, int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, st := range org.Stacks {
		if st.MetaID == id && target == st.Target {
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
func (d *Data) UpsertStack(orguuid resources.UUID, st Stack) (int64, error) {
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

	_, id, found := d.GetStackByMetaID(org, st.MetaID, st.Target)

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
func (d *Data) AppendPreviewLogs(org Org, stackPreviewID string, logs resources.CommandLogs) error {
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
func (d *Data) UpsertPreview(orguuid resources.UUID, p Preview) (string, error) {
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

	_, id, found := d.getPreview(org, p.ReviewRequest.Number, p.PushedAt)
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
		_, spIndex, spFound := d.getStackPreviewByMetaID(sp.MetaID, stackPreviews)
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
func (d *Data) UpdateStackPreview(org Org, stackPreviewID string, status string, changeset *resources.ChangesetDetails) (*StackPreview, bool) {
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
func (d *Data) GetDeployment(org *Org, id resources.UUID) (*Deployment, bool) {
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
func (d *Data) GetStackDrifts(orguuid resources.UUID, stackID int64) ([]Drift, error) {
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
func (d *Data) InsertDeployment(orgID resources.UUID, deploy Deployment) error {
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
	deploy.State.StackLogs = make(map[int64]resources.CommandLogs)
	deploy.State.StackStatusEvents = make(map[int64][]deployment.Status)
	for _, stackID := range deploy.Stacks {
		deploy.State.StackStatusEvents[stackID] = append(deploy.State.StackStatusEvents[stackID], deployment.Pending)
	}
	if org.Deployments == nil {
		org.Deployments = make(map[resources.UUID]*Deployment)
	}
	org.Deployments[deploy.UUID] = &deploy
	d.Orgs[org.Name] = org
	return nil
}

// FindDeploymentForCommit returns the deployment for the given commit.
func (d *Data) FindDeploymentForCommit(orgID resources.UUID, commitSHA string) (*Deployment, bool) {
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
func (d *Data) InsertDrift(orgID resources.UUID, drift Drift) (int, error) {
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
func (d *Data) SetDeploymentStatus(org Org, deploymentID resources.UUID, stackID int64, status deployment.Status) error {
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
func (d *Data) GetDeploymentEvents(orgID, deploymentID resources.UUID) (map[string][]deployment.Status, error) {
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
		metaid := org.Stacks[stackID].MetaID
		target := org.Stacks[stackID].Target
		eventsPerStack[target+"|"+metaid] = events
	}
	return eventsPerStack, nil
}

// InsertDeploymentLogs inserts logs for the given deployment.
func (d *Data) InsertDeploymentLogs(
	orgID resources.UUID,
	stackMetaID string,
	stackTarget string,
	deploymentID resources.UUID,
	logs resources.CommandLogs,
) error {
	org, found := d.GetOrg(orgID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	_, stackID, found := d.GetStackByMetaID(org, stackMetaID, stackTarget)
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
func (d *Data) GetDeploymentLogs(orgID resources.UUID, stackMetaID string, stackTarget string, deploymentID resources.UUID, fromLine int) (resources.CommandLogs, error) {
	org, found := d.GetOrg(orgID)
	if !found {
		return nil, errors.E(ErrNotExists, "org uuid %s", orgID)
	}
	_, stackID, found := d.GetStackByMetaID(org, stackMetaID, stackTarget)
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

// InsertOutput inserts the given output into the store.
func (d *Data) InsertOutput(orgUUID resources.UUID, output *resources.StoreOutput) error {
	org, found := d.GetOrg(orgUUID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgUUID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if org.Outputs == nil {
		org.Outputs = make(map[string]resources.StoreOutput)
	}
	key, err := encodeOutputPK(output.Key)
	if err != nil {
		return errors.E(err, "failed primary key constraint")
	}
	if _, exists := org.Outputs[key]; exists {
		return errors.E(ErrAlreadyExists, "output key %s", key)
	}
	output.ID = resources.UUID(uuid.New().String())
	output.CreatedAt = time.Now().UTC()
	output.UpdatedAt = output.CreatedAt
	org.Outputs[key] = *output
	d.Orgs[org.Name] = org
	return nil
}

// UpdateOutputValue updates the value of the output.
func (d *Data) UpdateOutputValue(orgUUID resources.UUID, id resources.UUID, newval string) error {
	org, found := d.GetOrg(orgUUID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgUUID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, output := range org.Outputs {
		if output.ID == id {
			output.Value = newval
			output.UpdatedAt = time.Now().UTC()
			org.Outputs[key] = output
			d.Orgs[org.Name] = org
			return nil
		}
	}
	return errors.E(ErrNotExists, "output id %s", id)
}

// DeleteOutput deletes the output with the given id.
func (d *Data) DeleteOutput(orgUUID resources.UUID, id resources.UUID) error {
	org, found := d.GetOrg(orgUUID)
	if !found {
		return errors.E(ErrNotExists, "org uuid %s", orgUUID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, output := range org.Outputs {
		if output.ID == id {
			delete(org.Outputs, key)
			d.Orgs[org.Name] = org
			return nil
		}
	}
	return errors.E(ErrNotExists, "output id %s", id)
}

// GetOutput retrieves the output for the given id.
func (d *Data) GetOutput(orgUUID resources.UUID, id resources.UUID) (resources.StoreOutput, error) {
	org, found := d.GetOrg(orgUUID)
	if !found {
		return resources.StoreOutput{}, errors.E(ErrNotExists, "org uuid %s", orgUUID)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, output := range org.Outputs {
		if output.ID == id {
			return output, nil
		}
	}
	return resources.StoreOutput{}, errors.E(ErrNotExists, "output id %s", id)
}

// GetOutputByKey retrieves the output for the given key.
func (d *Data) GetOutputByKey(orgUUID resources.UUID, key resources.StoreOutputKey) (resources.StoreOutput, error) {
	org, found := d.GetOrg(orgUUID)
	if !found {
		return resources.StoreOutput{}, errors.E(ErrNotExists, "org uuid %s", orgUUID)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	keystr, err := encodeOutputPK(key)
	if err != nil {
		return resources.StoreOutput{}, errors.E(err, "failed primary key constraint")
	}
	output, exists := org.Outputs[keystr]
	if !exists {
		return resources.StoreOutput{}, errors.E(ErrNotExists, "output key %s", keystr)
	}
	return output, nil
}

func (d *Data) getStackPreviewByMetaID(spMetaID string, stackPreviews []*StackPreview) (*StackPreview, int64, bool) {
	for i := range stackPreviews {
		if stackPreviews[i].MetaID == spMetaID {
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

func (d *Data) upsertReviewRequest(org Org, newRR *resources.ReviewRequest) {
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

func (d *Data) getPreview(org Org, rrNumber int, pushedAt int64) (Preview, int64, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, p := range org.Previews {
		if org.Previews[i].ReviewRequest.Number == rrNumber && org.Previews[i].PushedAt == pushedAt {
			return p, int64(i), true
		}
	}
	return Preview{}, 0, false
}

// Clone the organization.
func (org Org) Clone() Org {
	neworg := org // copy the non-pointer values

	// clones below are all shallow clones but enough for the mutation cases we handle.
	neworg.Deployments = maps.Clone(org.Deployments)
	neworg.Drifts = slices.Clone(org.Drifts)
	neworg.Stacks = slices.Clone(org.Stacks)
	neworg.Members = slices.Clone(org.Members)
	neworg.ReviewRequests = slices.Clone(org.ReviewRequests)
	neworg.Previews = slices.Clone(org.Previews)
	return neworg
}

// encodeOutputPK encodes the output primary key.
func encodeOutputPK(k resources.StoreOutputKey) (string, error) {
	if k.OrgUUID == "" {
		return "", errors.E("org uuid is required")
	}
	if k.Repository == "" {
		return "", errors.E("repository is required")
	}
	if k.StackMetaID == "" {
		return "", errors.E("stack meta id is required")
	}
	if k.Target == "" {
		return "", errors.E("stack target is required")
	}
	return string(k.OrgUUID) + "|" + k.Repository + "|" + k.StackMetaID + "|" + k.Target + "|" + k.Name, nil
}
