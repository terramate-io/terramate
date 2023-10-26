// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/stack"
)

type driftHandler struct {
	mu       sync.Mutex
	drifts   []cloud.DriftStackPayloadRequest
	statuses map[string]stack.Status // map of stack_meta_id -> status
}

func newDriftEndpoint() *driftHandler {
	return &driftHandler{
		statuses: make(map[string]stack.Status),
	}
}

func (dhandler *driftHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dhandler.mu.Lock()
	defer dhandler.mu.Unlock()

	params := httprouter.ParamsFromContext(r.Context())
	orguuid := params.ByName("orguuid")

	if orguuid == "" {
		w.WriteHeader(http.StatusInternalServerError)
		writeString(w, "expects an org uuid in the URL")
		return
	}

	defer justClose(r.Body)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		body, err := json.Marshal(dhandler.drifts)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
		write(w, body)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload cloud.DriftStackPayloadRequest
	err = json.Unmarshal(body, &payload)
	if err != nil {
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

	dhandler.drifts = append(dhandler.drifts, payload)
	dhandler.statuses[payload.Stack.MetaID] = payload.Status
	w.WriteHeader(http.StatusNoContent)
}
