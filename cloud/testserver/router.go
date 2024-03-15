// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package testserver provides fake Terramate Cloud endpoints for testing purposes.
package testserver

import (
	"net/http"
	"strings"

	"golang.org/x/exp/slices"

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
			writeString(w, " only supports terramate/.* User-Agents")
			return
		}
		fn(store, w, r, p)
	}
}

func handlerGithub(store *cloudstore.Data, fn Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "go-github/") {
			w.WriteHeader(http.StatusBadRequest)
			writeString(w, "only supports go-github/.* User-Agents")
			return
		}
		fn(store, w, r, p)
	}
}

// RouterAdd enables endpoints in an existing router.
func RouterAdd(store *cloudstore.Data, router *httprouter.Router, enabled map[string]bool) {
	if enabled[cloud.WellKnownCLIPath] {
		router.GET(cloud.WellKnownCLIPath, handler(store, GetWellKnown))
	}

	if enabled[cloud.UsersPath] {
		router.GET(cloud.UsersPath, handler(store, GetUsers))
	}

	if enabled[cloud.StacksPath] {
		router.GET(cloud.StacksPath+"/:orguuid", handler(store, GetStacks))
		router.POST(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", handler(store, PostDeploymentLogs))
		router.GET(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs", handler(store, GetDeploymentLogs))
		router.GET(cloud.StacksPath+"/:orguuid/:stackid/deployments/:deployment_uuid/logs/events", handler(store, GetDeploymentLogsEvents))

		router.GET(cloud.StacksPath+"/:orguuid/:stackid/drifts", handler(store, GetStackDrifts))

		// not a real TMC handler, only used by tests to populate the stacks state.
		router.PUT(cloud.StacksPath+"/:orguuid/:stackuuid", handler(store, PutStack))
	}

	if enabled[cloud.MembershipsPath] {
		router.GET(cloud.MembershipsPath, handler(store, GetMemberships))
	}

	if enabled[cloud.DeploymentsPath] {
		router.GET(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, GetDeployments))
		router.POST(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, PostDeployment))
		router.PATCH(cloud.DeploymentsPath+"/:orguuid/:deployuuid/stacks", handler(store, PatchDeployment))
	}

	if enabled[cloud.DriftsPath] {
		router.GET(cloud.DriftsPath+"/:orguuid/:stackid/:driftid", handler(store, GetDrift))
		router.POST(cloud.DriftsPath+"/:orguuid", handler(store, PostDrift))

		// test only
		router.GET(cloud.DriftsPath+"/:orguuid", handler(store, GetDrifts))
	}

	if enabled[cloud.PreviewsPath] {
		router.POST(cloud.PreviewsPath+"/:orguuid", handler(store, PostPreviews))
		router.PATCH(cloud.StackPreviewsPath+"/:orguuid/:stack_preview_id", handler(store, PatchStackPreviews))
		router.POST(cloud.StackPreviewsPath+"/:orguuid/:stack_preview_id/logs", handler(store, PostStackPreviewsLogs))
		router.GET(cloud.PreviewsPath+"/:orguuid/:preview_id", handler(store, GetPreview))
	}

	if enabled["github_api"] {
		router.GET("/repos/:owner/:repo/pulls/:pull_number", handlerGithub(store, GetPullRequest))
		router.GET("/repos/:owner/:repo/pulls/:pull_number/reviews", handlerGithub(store, ListReviews))
		router.GET("/repos/:owner/:repo/pulls/:pull_number/merge", handlerGithub(store, PullRequestIsMerged))
		router.GET("/repos/:owner/:repo/commits/:ref", handlerGithub(store, GetCommit))
		router.GET("/repos/:owner/:repo/commits/:ref/pulls", handlerGithub(store, ListPullRequestsWithCommit))
		router.GET("/repos/:owner/:repo/commits/:ref/check-runs", handlerGithub(store, ListCheckRunsForRef))
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
		cloud.WellKnownCLIPath: true,
		cloud.UsersPath:        true,
		cloud.MembershipsPath:  true,
		cloud.DeploymentsPath:  true,
		cloud.DriftsPath:       true,
		cloud.StacksPath:       true,
		cloud.PreviewsPath:     true,
		"github_api":           true,
	}
}

// DisableEndpoints the provided path endpoints.
func DisableEndpoints(paths ...string) map[string]bool {
	routes := map[string]bool{}
	for k, v := range EnableAllConfig() {
		if !slices.Contains(paths, k) {
			routes[k] = v
		}
	}
	return routes
}
