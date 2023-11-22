// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package testserver provides fake Terramate Cloud endpoints for testing purposes.
package testserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
)

type (
	// Route declares an HTTP route.
	Route struct {
		Path    string
		Handler Handler
	}

	// Custom declares a custom server config.
	Custom struct {
		Routes map[string]Route
	}

	// Handler is the testserver handler interface.
	Handler func(store *cloudstore.Data, w http.ResponseWriter, r *http.Request, p httprouter.Params)
)

// Router returns the default fake cloud router.
func Router(store *cloudstore.Data) *httprouter.Router {
	return RouterWith(store, EnableAllConfig())
}

// RouterWith returns the testserver router configuration only for the
// enabled endpoints.
func RouterWith(store *cloudstore.Data, enabled map[string]bool) *httprouter.Router {
	router := httprouter.New()
	RouterAdd(store, router, enabled)
	return router
}

func handler(store *cloudstore.Data, fn Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "terramate/") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fn(store, w, r, p)
	}
}

// RouterAdd enables endpoints in an existing router.
func RouterAdd(store *cloudstore.Data, router *httprouter.Router, enabled map[string]bool) {
	if enabled[cloud.UsersPath] {
		router.GET(cloud.UsersPath, handler(store, GetUsers))
	}

	if enabled[cloud.StacksPath] {
		router.GET(cloud.StacksPath+"/:orguuid", handler(store, GetStacks))
		router.POST(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", handler(store, PostDeploymentLogs))
		router.GET(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", handler(store, GetDeploymentLogs))
		router.GET(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs/events", handler(store, GetDeploymentLogsEvents))

		// not a real TMC handler, only used by tests to populate the stacks state.
		router.PUT(cloud.StacksPath+"/:orguuid/:stackuuid", handler(store, PutStack))
	}

	if enabled[cloud.MembershipsPath] {
		router.GET(cloud.MembershipsPath, handler(store, GetMemberships))
	}

	if enabled[cloud.DeploymentsPath] {
		router.GET(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, GetDeployments))
		router.POST(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, CreateDeployment))
		router.PATCH(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, PatchDeployment))
	}

	driftEndpoint := newDriftEndpoint()
	if enabled[cloud.DriftsPath] {
		router.Handler("POST", fmt.Sprintf("%s/:orguuid", cloud.DriftsPath), driftEndpoint)

		// test only
		router.Handler("GET", fmt.Sprintf("%s/:orguuid", cloud.DriftsPath), driftEndpoint)
	}
}

// RouterAddCustoms add custom routes to the fake server.
// This is used by very specific test cases which requires injection of custom
// errors in the server.
func RouterAddCustoms(router *httprouter.Router, store *cloudstore.Data, custom Custom) {
	for method, route := range custom.Routes {
		router.Handle(method, route.Path, handler(store, route.Handler))
	}
}

// EnableAllConfig returns a map that enables all cloud endpoints.
func EnableAllConfig() map[string]bool {
	return map[string]bool{
		cloud.UsersPath:       true,
		cloud.MembershipsPath: true,
		cloud.DeploymentsPath: true,
		cloud.DriftsPath:      true,
		cloud.StacksPath:      true,
	}
}
