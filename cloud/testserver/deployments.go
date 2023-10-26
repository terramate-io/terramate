// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
)

type deploymentHandler struct {
	mu          sync.Mutex
	nextStackID int64
	// as hacky as it can get:
	// map of organization -> (map of deployment_id -> (map of stack_id -> deployment))
	deployments map[string]map[string]map[int64]cloud.DeploymentStackRequest

	events map[string]map[string]map[string][]string
}

func newDeploymentEndpoint() *deploymentHandler {
	return &deploymentHandler{
		deployments: make(map[string]map[string]map[int64]cloud.DeploymentStackRequest),
		events:      make(map[string]map[string]map[string][]string),
	}
}

func (dhandler *deploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dhandler.mu.Lock()
	defer dhandler.mu.Unlock()

	params := httprouter.ParamsFromContext(r.Context())
	orguuid := params.ByName("orguuid")
	deployuuid := params.ByName("deployuuid")

	if !strings.HasPrefix(r.Header.Get("User-Agent"), "terramate/") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if dhandler.deployments[orguuid] == nil {
		dhandler.deployments[orguuid] = make(map[string]map[int64]cloud.DeploymentStackRequest)
		dhandler.events[orguuid] = make(map[string]map[string][]string)
	}
	if dhandler.deployments[orguuid][deployuuid] == nil {
		dhandler.deployments[orguuid][deployuuid] = make(map[int64]cloud.DeploymentStackRequest)
		dhandler.events[orguuid][deployuuid] = make(map[string][]string)
	}

	w.Header().Add("Content-Type", "application/json")

	if strings.HasSuffix(r.URL.Path, "/events") {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		events := dhandler.events[orguuid][deployuuid]
		data, err := json.Marshal(events)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		write(w, data)
		return
	}

	if r.Method == "GET" {
		// this is not supported by the real server but used as testing purposes.
		deploymentInfo := dhandler.deployments[orguuid][deployuuid]
		data, err := json.Marshal(deploymentInfo)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		write(w, data)
		return
	}

	if r.Method == "POST" {
		defer func() { _ = r.Body.Close() }()
		data, _ := io.ReadAll(r.Body)
		var p cloud.DeploymentStacksPayloadRequest
		err := json.Unmarshal(data, &p)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		err = p.Validate()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeErr(w, err)
			return
		}

		// deployment commit_sha is not required but must be present in all test cases.
		for _, st := range p.Stacks {
			if st.CommitSHA == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`commit_sha is missing`))
				return
			}
		}

		res := cloud.DeploymentStacksResponse{}
		for _, s := range p.Stacks {
			next := atomic.LoadInt64(&dhandler.nextStackID)
			res = append(res, cloud.DeploymentStackResponse{
				StackID:     int(next),
				StackMetaID: s.MetaID,
				Status:      deployment.Pending,
			})

			atomic.AddInt64(&dhandler.nextStackID, 1)

			s.DeploymentStatus = deployment.Pending
			dhandler.deployments[orguuid][deployuuid][next] = s
			dhandler.events[orguuid][deployuuid][s.MetaID] = append(dhandler.events[orguuid][deployuuid][s.MetaID], s.DeploymentStatus.String())
		}
		data, _ = json.Marshal(res)
		write(w, data)
		return
	}

	if r.Method == "PATCH" {
		defer func() { _ = r.Body.Close() }()
		data, _ := io.ReadAll(r.Body)
		var updateStacks cloud.UpdateDeploymentStacks
		err := json.Unmarshal(data, &updateStacks)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeErr(w, err)
			return
		}

		for _, s := range updateStacks.Stacks {
			if gotStack := dhandler.deployments[orguuid][deployuuid][int64(s.StackID)]; gotStack.MetaID != "" {
				gotStack.DeploymentStatus = s.Status
				dhandler.deployments[orguuid][deployuuid][int64(s.StackID)] = gotStack
				dhandler.events[orguuid][deployuuid][gotStack.MetaID] = append(dhandler.events[orguuid][deployuuid][gotStack.MetaID], s.Status.String())
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				writeString(w, `{"error": "stack not found"}`)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
