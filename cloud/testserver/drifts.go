// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud/api/drift"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
)

// GetDrift implements the /v1/drifts/:orguuid/:stackid/:driftid endpoint.
func GetDrift(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	orguuid := resources.UUID(params.ByName("orguuid"))
	stackid, err := strconv.Atoi64(params.ByName("stackid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, errors.E(err, "invalid stackid"))
		return
	}

	driftid, err := strconv.Atoi64(params.ByName("driftid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, errors.E(err, "invalid driftid"))
		return
	}

	drifts, err := store.GetStackDrifts(orguuid, stackid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	for _, drift := range drifts {
		if drift.ID == driftid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			marshalWrite(w, resources.Drift{
				ID:       drift.ID,
				Status:   drift.Status,
				Details:  drift.Changeset,
				Metadata: drift.Metadata,
			})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

// PostDrift implements the POST /v2/drifts/:orguuid endpoint.
func PostDrift(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	orgUUID := p.ByName("orguuid")
	org, found := store.GetOrg(resources.UUID(orgUUID))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var payload resources.DriftCheckRunStartPayloadRequest
	if err = json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	err = payload.Validate()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	// NOTE(i4k): metadata is not required but must be present in all test cases
	if err := validateMetadata(payload.Metadata); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	if payload.Stack.Target == "" {
		payload.Stack.Target = "default"
	}
	st, _, found := store.GetStackByMetaID(org, payload.Stack.MetaID, payload.Stack.Target)
	if !found {
		st = cloudstore.Stack{
			Stack: payload.Stack,
			State: cloudstore.NewState(),
		}
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	_, err = store.InsertDrift(resources.UUID(orgUUID), cloudstore.Drift{
		StackMetaID: payload.Stack.MetaID,
		StackTarget: payload.Stack.Target,
		Metadata:    payload.Metadata,
		Status:      drift.Running,
		Command:     payload.Command,
		StartedAt:   payload.StartedAt,
		UUID:        resources.UUID(uuid.String()),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var ok bool
	st.State.Status, ok = stateTable()[drift.Running][st.State.DeploymentStatus]
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E("invalid stack status: %s %s", drift.Running.String(), st.State.DeploymentStatus.String()))
		return
	}

	_, err = store.UpsertStack(resources.UUID(orgUUID), st)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshalWrite(w, resources.DriftCheckRunStartResponse{
		DriftUUID: resources.UUID(uuid.String()),
	})
}

// PatchDrift implements the PATCH /v2/drifts/:orguuid/:driftuuid endpoint.
func PatchDrift(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	orgUUID := p.ByName("orguuid")
	org, found := store.GetOrg(resources.UUID(orgUUID))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}
	driftUUID := p.ByName("driftuuid")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	justClose(r.Body)

	var payload resources.UpdateDriftPayloadRequest
	if err = json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	err = payload.Validate()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	driftCheck, updated := store.UpdateDrift(&org, resources.UUID(driftUUID), payload.Status, payload.Changeset, payload.UpdatedAt)
	if !updated {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, fmt.Errorf("drift not found %s in org %s", driftUUID, org.UUID))
		return
	}

	st, _, found := store.GetStackByMetaID(org, driftCheck.StackMetaID, driftCheck.StackTarget)
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, fmt.Errorf("stack not found %s (%s) in org %s", driftCheck.StackMetaID, driftCheck.StackTarget, org.UUID))
		return
	}

	var ok bool
	st.State.Status, ok = stateTable()[driftCheck.Status][st.State.DeploymentStatus]
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E("invalid stack status: %s %s", driftCheck.Status.String(), st.State.DeploymentStatus.String()))
		return
	}

	_, err = store.UpsertStack(resources.UUID(orgUUID), st)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetDrifts implements the GET /v2/drifts/:orguuid endpoint.
// Note: this is not a real endpoint.
func GetDrifts(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	orgUUID := p.ByName("orguuid")
	org, found := store.GetOrg(resources.UUID(orgUUID))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}
	res := resources.DriftsWithStacks{}
	for _, drift := range org.Drifts {
		st, _, ok := store.GetStackByMetaID(org, drift.StackMetaID, drift.StackTarget)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			writeString(w, fmt.Sprintf("stack not found %s:%s", drift.StackMetaID, drift.StackTarget))
			return
		}
		res = append(res, resources.DriftWithStack{
			Stack: st.Stack,
			Drift: resources.Drift{
				ID:       drift.ID,
				UUID:     drift.UUID,
				Metadata: drift.Metadata,
				Details:  drift.Changeset,
				Status:   drift.Status,
			},
			StartedAt:  drift.StartedAt,
			FinishedAt: drift.FinishedAt,
		})
	}

	// Return them sorted by drift ID as retrieving from map is not predictable.
	// The drift.ID is the way to know the order of insertion.
	slices.SortFunc(res, func(a, b resources.DriftWithStack) int {
		return cmp.Compare(a.ID, b.ID)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	marshalWrite(w, res)
}

// InsertDriftLogs implements the POST /v2/drifts/:orguuid/:driftuuid/logs endpoint.
func InsertDriftLogs(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	orgUUID := p.ByName("orguuid")
	org, found := store.GetOrg(resources.UUID(orgUUID))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}
	driftUUID := resources.UUID(p.ByName("driftuuid"))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	justClose(r.Body)

	var logs resources.CommandLogs
	err = json.Unmarshal(body, &logs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = store.InsertDriftLogs(org.UUID, driftUUID, logs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetDriftLogs implements the GET /v2/drifts/:orguuid/:driftuuid/logs endpoint.
func GetDriftLogs(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	orgUUID := p.ByName("orguuid")
	org, found := store.GetOrg(resources.UUID(orgUUID))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}
	driftUUID := resources.UUID(p.ByName("driftuuid"))

	logs, err := store.GetDriftLogs(org.UUID, driftUUID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	data, err := json.MarshalIndent(logs, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	write(w, data)
}
