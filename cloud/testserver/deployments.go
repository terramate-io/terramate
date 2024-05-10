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
	"github.com/terramate-io/terramate/cloud/stack"
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

// PostDeployment is the POST /deployments handler.
func PostDeployment(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	var rPayload cloud.DeploymentStacksPayloadRequest
	if err := json.Unmarshal(data, &rPayload); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E(err, "failed to unmarshal data: %s", data))
		return
	}

	if err := rPayload.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	// NOTE(i4k): The metadata is not required but must be present in all test cases.
	err = validateMetadata(rPayload.Metadata)
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
			writeString(w, `commit_sha is missing`)
			return
		}
		stackCommands[st.MetaID] = st.DeploymentCommand
	}

	orguuid := cloud.UUID(p.ByName("orguuid"))
	deployuuid := cloud.UUID(p.ByName("deployuuid"))

	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "org not found")
		return
	}

	var stackIDs []int64
	res := cloud.DeploymentStacksResponse{}
	for _, s := range rPayload.Stacks {
		state := cloudstore.NewState()
		gotStack, _, found := store.GetStackByMetaID(org, s.Stack.MetaID)
		if found {
			state = gotStack.State
		}
		state.DeploymentStatus = deployment.Pending
		stackid, err := store.UpsertStack(orguuid, cloudstore.Stack{
			Stack: s.Stack,
			State: state,
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
	data, _ := io.ReadAll(r.Body)
	var updateStacks cloud.UpdateDeploymentStacks
	if err := json.Unmarshal(data, &updateStacks); err != nil {
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
		if s.Status.IsFinalState() {
			st, ok := store.GetStack(org, s.StackID)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var status stack.Status
			switch s.Status {
			case deployment.OK:
				status = stack.OK
			case deployment.Failed, deployment.Canceled:
				status = stack.Failed
			default:
				panic("unreachable")
			}

			st.State.Status = status
			st.State.DeploymentStatus = s.Status
			_, err = store.UpsertStack(org.UUID, st)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				writeErr(w, err)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
