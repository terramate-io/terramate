// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
)

// GetDeployments is the GET /deployments handler.
func GetDeployments(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	orguuid := cloud.UUID(p.ByName("orguuid"))
	deployuuid := cloud.UUID(p.ByName("deployuuid"))

	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	deploymentInfo, found := store.GetDeployment(&org, deployuuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	marshalWrite(w, deploymentInfo)
}

// CreateDeployment is the POST /deployments handler.
func CreateDeployment(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	defer func() { _ = r.Body.Close() }()
	data, _ := io.ReadAll(r.Body)
	var rPayload cloud.DeploymentStacksPayloadRequest
	err := json.Unmarshal(data, &rPayload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E(err, "failed to unmarshal data: %s", data))
		return
	}

	err = rPayload.Validate()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	stackCommands := map[string]string{}

	// deployment commit_sha is not required but must be present in all test cases.
	// TODO(i4k): review this!!!
	for _, st := range rPayload.Stacks {
		if st.CommitSHA == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`commit_sha is missing`))
			return
		}
		stackCommands[st.MetaID] = st.DeploymentCommand
	}

	orguuid := cloud.UUID(p.ByName("orguuid"))
	deployuuid := cloud.UUID(p.ByName("deployuuid"))

	var stackIDs []int
	res := cloud.DeploymentStacksResponse{}
	for _, s := range rPayload.Stacks {
		stackid, err := store.UpsertStack(orguuid, cloudstore.Stack{
			Stack: s.Stack,
			State: cloudstore.StackState{
				DeploymentStatus: deployment.Pending,
			},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		res = append(res, cloud.DeploymentStackResponse{
			StackID:     stackid,
			StackMetaID: s.MetaID,
			Status:      deployment.Pending,
		})
		stackIDs = append(stackIDs, stackid)
	}

	err = store.InsertDeployment(orguuid, cloudstore.Deployment{
		UUID:          deployuuid,
		Workdir:       rPayload.Workdir.String(),
		Stacks:        stackIDs,
		StackCommands: stackCommands,
		Metadata:      rPayload.Metadata,
		ReviewRequest: rPayload.ReviewRequest,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	marshalWrite(w, res)
}

// PatchDeployment is the PATCH /deployments handler.
func PatchDeployment(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	defer func() { _ = r.Body.Close() }()
	data, _ := io.ReadAll(r.Body)
	var updateStacks cloud.UpdateDeploymentStacks
	err := json.Unmarshal(data, &updateStacks)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E(err, "failed to unmarshal data: %s", data))
		return
	}

	orguuid := cloud.UUID(p.ByName("orguuid"))
	deployuuid := cloud.UUID(p.ByName("deployuuid"))

	org, found := store.GetOrg(orguuid)
	if !found {
		writeErr(w, errors.E("org uuid %s does not exists", orguuid))
		return
	}

	for _, s := range updateStacks.Stacks {
		err := store.SetDeploymentStatus(org, deployuuid, s.StackID, s.Status)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}
		st, _ := store.GetStack(org, s.StackID)
		var ok bool
		st.State.Status, ok = stateTable()[st.State.DriftStatus][st.State.DeploymentStatus]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, invalidStackStateError(st))
			return
		}
		_, err = store.UpsertStack(org.UUID, st)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
