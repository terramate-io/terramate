// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package testserver provides fake Terramate Cloud endpoints for testing purposes.
package testserver

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/terramate-io/terramate/cloud"
)

// DefaultOrgUUID is the test organization UUID.
const DefaultOrgUUID = "0000-1111-2222-3333"

type (
	// Route declares an HTTP route.
	Route struct {
		Path    string
		Handler http.Handler
	}

	// Custom declares a custom server config.
	Custom struct {
		Routes map[string]Route
	}
)

// Router returns the default fake cloud router.
func Router() *httprouter.Router {
	return RouterWith(EnableAllConfig())
}

// RouterWith returns the testserver router configuration only for the
// enabled endpoints.
func RouterWith(enabled map[string]bool) *httprouter.Router {
	router := httprouter.New()
	RouterAdd(router, enabled)
	return router
}

// RouterAdd enables endpoints in an existing router.
func RouterAdd(router *httprouter.Router, enabled map[string]bool) {
	if enabled[cloud.UsersPath] {
		router.Handler("GET", cloud.UsersPath, newUserEndpoint())
	}

	if enabled[cloud.StacksPath] {
		stackHandler := newStackEndpoint()
		router.Handler("GET", cloud.StacksPath+"/:orguuid", stackHandler)
		router.Handler("POST", cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", stackHandler)
		router.Handler("GET", cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", stackHandler)
		router.Handler("GET", cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs/events", stackHandler)

		// not a real TMC handler, only used by tests to populate the stacks state.
		router.Handler("PUT", cloud.StacksPath+"/:orguuid/:stackid", stackHandler)
	}

	if enabled[cloud.MembershipsPath] {
		router.Handler("GET", cloud.MembershipsPath, newMembershipEndpoint())
	}

	deploymentEndpoint := newDeploymentEndpoint()
	if enabled[cloud.DeploymentsPath] {
		router.Handler("GET", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
		router.Handler("POST", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
		router.Handler("PATCH", fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath), deploymentEndpoint)
	}

	driftEndpoint := newDriftEndpoint()
	if enabled[cloud.DriftsPath] {
		router.Handler("POST", fmt.Sprintf("%s/:orguuid", cloud.DriftsPath), driftEndpoint)

		// test only
		router.Handler("GET", fmt.Sprintf("%s/:orguuid", cloud.DriftsPath), driftEndpoint)
	}

	// test endpoint always enabled
	router.Handler("GET", fmt.Sprintf("%s/:orguuid/:deployuuid/events", cloud.DeploymentsPath), deploymentEndpoint)
}

// RouterAddCustoms add custom routes to the fake server.
// This is used by very specific test cases which requires injection of custom
// errors in the server.
func RouterAddCustoms(router *httprouter.Router, custom Custom) {
	for method, route := range custom.Routes {
		router.Handler(method, route.Path, route.Handler)
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
