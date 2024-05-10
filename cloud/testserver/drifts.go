// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/strconv"
)

// GetDrift implements the /v1/drifts/:orguuid/:stackid/:driftid endpoint.
func GetDrift(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	orguuid := cloud.UUID(params.ByName("orguuid"))
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
			marshalWrite(w, cloud.Drift{
				ID:       drift.ID,
				Status:   drift.Status,
				Details:  drift.Details,
				Metadata: drift.Metadata,
			})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

// PostDrift implements the POST /v1/drifts/:orguuid endpoint.
func PostDrift(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	orguuid := p.ByName("orguuid")
	org, found := store.GetOrg(cloud.UUID(orguuid))
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

	var payload cloud.DriftStackPayloadRequest
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

	st, _, found := store.GetStackByMetaID(org, payload.Stack.MetaID)
	if !found {
		st = cloudstore.Stack{
			Stack: payload.Stack,
			State: cloudstore.NewState(),
		}
	}
	_, err = store.InsertDrift(cloud.UUID(orguuid), cloudstore.Drift{
		StackMetaID: payload.Stack.MetaID,
		Metadata:    payload.Metadata,
		Details:     payload.Details,
		Status:      payload.Status,
		Command:     payload.Command,
		StartedAt:   payload.StartedAt,
		FinishedAt:  payload.FinishedAt,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var ok bool
	st.State.Status, ok = stateTable()[payload.Status][st.State.DeploymentStatus]
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E("invalid stack status: %s %s", payload.Status.String(), st.State.DeploymentStatus.String()))
		return
	}

	_, err = store.UpsertStack(cloud.UUID(orguuid), st)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetDrifts implements the GET /drifts/:orguuid endpoint.
// Note: this is not a real endpoint.
func GetDrifts(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	orguuid := p.ByName("orguuid")
	org, found := store.GetOrg(cloud.UUID(orguuid))
	if !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "organization not found")
		return
	}
	res := cloud.DriftStackPayloadRequests{}
	for _, drift := range org.Drifts {
		st, _, ok := store.GetStackByMetaID(org, drift.StackMetaID)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			writeString(w, fmt.Sprintf("stack not found %s", drift.StackMetaID))
			return
		}
		res = append(res, cloud.DriftStackPayloadRequest{
			Stack:      st.Stack,
			Status:     drift.Status,
			Metadata:   drift.Metadata,
			Details:    drift.Details,
			Command:    drift.Command,
			StartedAt:  drift.StartedAt,
			FinishedAt: drift.FinishedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	marshalWrite(w, res)
}
