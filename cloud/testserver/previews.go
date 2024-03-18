// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
)

// GetPreview returns details about a preview. This is a convenience endpoint
// for asserting the required state in tests (it does not actually exist in the TMC API).
func GetPreview(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, p httprouter.Params) {
	orguuid := cloud.UUID(p.ByName("orguuid"))
	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}

	previewID := p.ByName("preview_id")
	preview, found := store.GetPreviewByID(org, previewID)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(preview); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
}

// PostStackPreviewsLogs is the POST /v1/stack_previews/:stack_preview_id/logs handler.
func PostStackPreviewsLogs(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	orguuid := cloud.UUID(p.ByName("orguuid"))
	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "organization not found")
		return
	}

	bodyData, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	justClose(r.Body)

	var logs cloud.CommandLogs
	err = json.Unmarshal(bodyData, &logs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	if err := logs.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	stackPreviewID := p.ByName("stack_preview_id")
	err = store.AppendPreviewLogs(org, stackPreviewID, logs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PatchStackPreviews is the PATCH /v1/stack_previews/:stack_preview_id handler.
func PatchStackPreviews(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var rPayload cloud.UpdateStackPreviewPayloadRequest
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

	orguuid := cloud.UUID(p.ByName("orguuid"))
	stackPreviewID := p.ByName("stack_preview_id")

	org, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "org not found")
		return
	}

	if _, found := store.UpdateStackPreview(org, stackPreviewID, rPayload.Status, rPayload.ChangesetDetails); !found {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, errors.E(err, "unable to find stack_preview_id: %s", stackPreviewID))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PostPreviews is the POST /v1/previews handler.
func PostPreviews(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var rPayload cloud.CreatePreviewPayloadRequest
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

	orguuid := cloud.UUID(p.ByName("orguuid"))

	_, found := store.GetOrg(orguuid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeString(w, "org not found")
		return
	}

	previewID, err := store.UpsertPreview(orguuid, cloudstore.Preview{
		UpdatedAt:       rPayload.UpdatedAt,
		PushedAt:        rPayload.PushedAt,
		CommitSHA:       rPayload.CommitSHA,
		Technology:      rPayload.Technology,
		TechnologyLayer: rPayload.TechnologyLayer,
		ReviewRequest:   rPayload.ReviewRequest,
		Metadata:        rPayload.Metadata,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	res := cloud.CreatePreviewResponse{
		PreviewID: previewID,
	}

	for _, s := range rPayload.Stacks {
		_, err := store.UpsertStack(orguuid, cloudstore.Stack{
			Stack: s.Stack,
			State: cloudstore.NewState(),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		org, found := store.GetOrg(orguuid)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			writeString(w, "org not found")
			return
		}

		stackPreviewID, err := store.UpsertStackPreview(org, previewID, &cloudstore.StackPreview{
			Stack: cloudstore.Stack{
				Stack: s.Stack,
				State: cloudstore.NewState(),
			},
			Status: s.PreviewStatus,
			Cmd:    s.Cmd,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		res.Stacks = append(res.Stacks, cloud.ResponsePreviewStack{
			MetaID:         s.MetaID,
			StackPreviewID: stackPreviewID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	marshalWrite(w, res)
}
