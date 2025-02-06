// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"

	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/errors"
)

// PostStoreOutput implements the /v1/store/:orguuid/outputs endpoint.
func PostStoreOutput(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	orgUUID := cloud.UUID(params.ByName("orguuid"))
	if orgUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "orguuid is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	var outputRequest cloud.StoreOutputRequest
	err = json.Unmarshal(body, &outputRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	if err := outputRequest.Validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeErr(w, err)
		return
	}

	target := outputRequest.Key.Target
	if target == "" {
		target = "default"
	}

	output := cloud.StoreOutput{
		StoreOutputKey: cloud.StoreOutputKey{
			OrgUUID:     orgUUID,
			Repository:  outputRequest.Key.Repository,
			StackMetaID: outputRequest.Key.StackMetaID,
			Target:      target,
			Name:        outputRequest.Key.Name,
		},
		Value: outputRequest.Value,
	}

	err = store.InsertOutput(orgUUID, &output)
	if err != nil {
		if errors.Is(err, errors.E(cloudstore.ErrAlreadyExists)) {
			w.WriteHeader(http.StatusConflict)
		} else if errors.Is(err, errors.E(cloudstore.ErrNotExists)) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		writeErr(w, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		log.Err(err).Msg("failed to encode output")
	}
}

// GetStoreOutput implements the GET /v1/store/:orguuid/outputs/:id endpoint.
func GetStoreOutput(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	orgUUID := cloud.UUID(params.ByName("orguuid"))
	if orgUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "orguuid is required")
		return
	}

	outputID := cloud.UUID(params.ByName("id"))
	if outputID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "id is required")
		return
	}

	output, err := store.GetOutput(orgUUID, outputID)
	if err != nil {
		if errors.Is(err, errors.E(cloudstore.ErrNotExists)) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		writeErr(w, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		log.Err(err).Msg("failed to encode output")
	}
}

// PutStoreOutputValue implements the PUT /v1/store/:orguuid/outputs/:id/value endpoint.
func PutStoreOutputValue(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	orgUUID := cloud.UUID(params.ByName("orguuid"))
	if orgUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "orguuid is required")
		return
	}

	outputID := cloud.UUID(params.ByName("id"))
	if outputID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "id is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	err = store.UpdateOutputValue(orgUUID, outputID, string(body))
	if err != nil {
		if errors.Is(err, errors.E(cloudstore.ErrNotExists)) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		writeErr(w, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteStoreOutput implements the DELETE /v1/store/:orguuid/outputs/:id endpoint.
func DeleteStoreOutput(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
	orgUUID := cloud.UUID(params.ByName("orguuid"))
	if orgUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "orguuid is required")
		return
	}

	outputID := cloud.UUID(params.ByName("id"))
	if outputID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeString(w, "id is required")
		return
	}

	err := store.DeleteOutput(orgUUID, outputID)
	if err != nil {
		if errors.Is(err, errors.E(cloudstore.ErrNotExists)) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		writeErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
