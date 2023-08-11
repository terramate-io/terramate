// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package testserver provides fake Terramate Cloud endpoints for testing purposes.
package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/deployment"
	"github.com/terramate-io/terramate/cloud/stack"
)

// DefaultOrgUUID is the test organization UUID.
const DefaultOrgUUID = "0000-1111-2222-3333"

func (orgHandler *membershipHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write([]byte(
		fmt.Sprintf(`[
		{
			"org_name": "terramate-io",
			"org_display_name": "Terramate",
			"org_uuid": "%s",
			"status": "active"
		}
	]`, DefaultOrgUUID),
	))
}

func (userHandler *userHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write([]byte(
		`{
			    "email": "batman@example.com",
			    "display_name": "batman",
				"job_title": "entrepreneur"
			}`,
	))
}

func (dhandler *deploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		_, _ = w.Write(data)
		return
	}

	if r.Method == "GET" {
		// this is not supported by the real server but used as testing purposes.
		deploymentInfo := dhandler.deployments[orguuid][deployuuid]
		data, err := json.Marshal(deploymentInfo)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		_, _ = w.Write(data)
		return
	}

	if r.Method == "POST" {
		defer func() { _ = r.Body.Close() }()
		data, _ := io.ReadAll(r.Body)
		var p cloud.DeploymentStacksPayloadRequest
		err := json.Unmarshal(data, &p)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		err = p.Validate()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
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
		_, _ = w.Write(data)
		return
	}

	if r.Method == "PATCH" {
		defer func() { _ = r.Body.Close() }()
		data, _ := io.ReadAll(r.Body)
		var updateStacks cloud.UpdateDeploymentStacks
		err := json.Unmarshal(data, &updateStacks)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		for _, s := range updateStacks.Stacks {
			if gotStack := dhandler.deployments[orguuid][deployuuid][int64(s.StackID)]; gotStack.MetaID != "" {
				gotStack.DeploymentStatus = s.Status
				dhandler.deployments[orguuid][deployuuid][int64(s.StackID)] = gotStack
				dhandler.events[orguuid][deployuuid][gotStack.MetaID] = append(dhandler.events[orguuid][deployuuid][gotStack.MetaID], s.Status.String())
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "stack not found"}`))
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (handler *stackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	orguuid := params.ByName("orguuid")
	filterStatusStr := r.FormValue("status")
	filterStatus := stack.AllFilter

	if filterStatusStr != "" && filterStatusStr != "unhealthy" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if filterStatusStr == "unhealthy" {
		filterStatus = stack.UnhealthyFilter
	}

	w.Header().Add("Content-Type", "application/json")

	if r.Method == "GET" {
		var resp cloud.StacksResponse
		var stacks []cloud.Stack
		stacksMap, ok := handler.stacks[orguuid]
		if !ok {
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
			return
		}
		for _, st := range stacksMap {
			if stack.FilterStatus(st.Status)&filterStatus != 0 {
				stacks = append(stacks, st)
			}
		}

		sort.Slice(stacks, func(i, j int) bool {
			return stacks[i].ID < stacks[j].ID
		})

		resp.Stacks = stacks
		data, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("marshaling error"))
		}
		_, _ = w.Write(data)
		return
	}

	if r.Method == "PUT" {
		stackIDStr := params.ByName("stackid")
		if stackIDStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		stackid, err := strconv.Atoi(stackIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
		}
		bodyData, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_ = r.Body.Close()

		var st cloud.Stack
		err = json.Unmarshal(bodyData, &st)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		if stackid != st.ID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, ok := handler.stacks[orguuid]; !ok {
			handler.stacks[orguuid] = make(map[int]cloud.Stack)
		}
		if _, ok := handler.statuses[orguuid]; !ok {
			handler.statuses[orguuid] = make(map[int]stack.Status)
		}

		handler.stacks[orguuid][stackid] = st
		handler.statuses[orguuid][stackid] = st.Status
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func newStackEndpoint() *stackHandler {
	return &stackHandler{
		stacks:   make(map[string]map[int]cloud.Stack),
		statuses: make(map[string]map[int]stack.Status),
	}
}

func newDeploymentEndpoint() *deploymentHandler {
	return &deploymentHandler{
		deployments: make(map[string]map[string]map[int64]cloud.DeploymentStackRequest),
		events:      make(map[string]map[string]map[string][]string),
	}
}

// Router returns the default fake cloud router.
func Router() *httprouter.Router {
	return RouterWith(EnableAllConfig())
}

// RouterWith returns the testserver router configuration only for the
// enabled endpoints.
func RouterWith(enabled map[string]bool) *httprouter.Router {
	router := httprouter.New()

	if enabled[cloud.UsersPath] {
		router.Handler("GET", cloud.UsersPath, &userHandler{})
	}

	if enabled[cloud.StacksPath] {
		stackHandler := newStackEndpoint()
		router.Handler("GET", cloud.StacksPath+"/:orguuid", stackHandler)

		// not a real TMC handler, only used by tests to populate the stacks state.
		router.Handler("PUT", cloud.StacksPath+"/:orguuid/:stackid", stackHandler)
	}

	if enabled[cloud.MembershipsPath] {
		router.Handler("GET", cloud.MembershipsPath, &membershipHandler{})
	}

	deploymentEndpoint := newDeploymentEndpoint()
	if enabled[cloud.DeploymentsPath] {
		router.Handler("GET", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
		router.Handler("POST", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
		router.Handler("PATCH", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
	}

	// test endpoint always enabled
	router.Handler("GET", fmt.Sprintf("%s/:orguuid/:deployuuid/events", cloud.DeploymentsPath), deploymentEndpoint)
	return router
}

type (
	userHandler       struct{}
	membershipHandler struct{}
	stackHandler      struct {
		stacks   map[string]map[int]cloud.Stack
		statuses map[string]map[int]stack.Status
	}
	deploymentHandler struct {
		nextStackID int64
		// as hacky as it can get:
		// map of organization -> (map of deployment_id -> (map of stack_id -> deployment))
		deployments map[string]map[string]map[int64]cloud.DeploymentStackRequest

		events map[string]map[string]map[string][]string
	}
)

// EnableAllConfig returns a map that enables all cloud endpoints.
func EnableAllConfig() map[string]bool {
	return map[string]bool{
		cloud.UsersPath:       true,
		cloud.MembershipsPath: true,
		cloud.DeploymentsPath: true,
		cloud.StacksPath:      true,
	}
}
