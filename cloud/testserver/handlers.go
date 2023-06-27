// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package testserver provides fake Terramate Cloud endpoints for testing purposes.
package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
)

// DefaultOrgUUID is the test organization UUID.
const DefaultOrgUUID = "0000-1111-2222-3333"

func (orgHandler *organizationHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write([]byte(
		fmt.Sprintf(`[
		{
			"org_name": "terramate-io",
			"org_display_name": "Terramate",
			"org_uuid": "%s"
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

		res := cloud.DeploymentStacksResponse{}
		for _, s := range p.Stacks {
			next := atomic.LoadInt64(&dhandler.nextStackID)
			res = append(res, cloud.DeploymentStackResponse{
				StackID:     int(next),
				StackMetaID: s.MetaID,
				Status:      cloud.Pending,
			})

			atomic.AddInt64(&dhandler.nextStackID, 1)

			s.Status = cloud.Pending
			dhandler.deployments[orguuid][deployuuid][next] = s
			dhandler.events[orguuid][deployuuid][s.MetaID] = append(dhandler.events[orguuid][deployuuid][s.MetaID], s.Status.String())
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
				gotStack.Status = s.Status
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

// newDeploymentEndpoint returns a new fake deployment endpoint.
func newDeploymentEndpoint() *deploymentHandler {
	return &deploymentHandler{
		deployments: make(map[string]map[string]map[int64]cloud.DeploymentStackRequest),
		events:      make(map[string]map[string]map[string][]string),
	}
}

// Router returns the testserver router configuration.
func Router() *httprouter.Router {
	router := httprouter.New()
	router.Handler("GET", "/v1/users", &userHandler{})
	router.Handler("GET", "/v1/organizations", &organizationHandler{})

	deploymentEndpoint := newDeploymentEndpoint()
	router.Handler("GET", "/v1/deployments/:orguuid/:deployuuid/stacks", deploymentEndpoint)
	router.Handler("POST", "/v1/deployments/:orguuid/:deployuuid/stacks", deploymentEndpoint)
	router.Handler("PATCH", "/v1/deployments/:orguuid/:deployuuid/stacks", deploymentEndpoint)
	router.Handler("GET", "/v1/deployments/:orguuid/:deployuuid/events", deploymentEndpoint)
	return router
}

type (
	userHandler         struct{}
	organizationHandler struct{}
	deploymentHandler   struct {
		nextStackID int64
		// as hacky as it can get:
		// map of organization -> (map of deployment_id -> (map of stack_id -> deployment))
		deployments map[string]map[string]map[int64]cloud.DeploymentStackRequest

		events map[string]map[string]map[string][]string
	}
)
