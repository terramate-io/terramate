// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudSyncUIMode(t *testing.T) {
	type subtestcase struct {
		name   string
		cmd    []string
		want   runExpected
		uimode cli.UIMode
	}

	type testcase struct {
		name            string
		layout          []string
		endpoints       map[string]bool
		customEndpoints testserver.Custom

		subcases []subtestcase
	}

	writeJSON := func(w http.ResponseWriter, str string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(str))
	}

	const invalidUserData = `{
		"email": "batman@example.com",
		"display_name": "",
		"job_title": ""
	}`

	const fatalErr = `FTL ` + string(clitest.ErrCloud)

	for _, tc := range []testcase{
		{
			name: "/v1/users is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       false,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudOnboardingIncomplete),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							string(clitest.ErrCloudOnboardingIncomplete),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudOnboardingIncomplete),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							string(clitest.ErrCloudOnboardingIncomplete),
							string(clitest.CloudDisablingMessage),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`FTL failed to load credentials: ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`FTL failed to load credentials: ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
			},
		},
		{
			name: "/v1/users returns unexpected payload",
			endpoints: map[string]bool{
				cloud.UsersPath:       false,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.UsersPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, invalidUserData)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							string(cloud.ErrUnexpectedResponseBody),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							string(cloud.ErrUnexpectedResponseBody),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							string(cloud.ErrUnexpectedResponseBody),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							string(cloud.ErrUnexpectedResponseBody),
							clitest.CloudDisablingMessage,
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + string(cloud.ErrUnexpectedResponseBody),
							`FTL failed to load credentials`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + string(cloud.ErrUnexpectedResponseBody),
							`FTL failed to load credentials`,
						},
					},
				},
			},
		},
		{
			name: "/v1/memberships is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status:      1,
						StderrRegex: fatalErr,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegex: string(clitest.CloudDisablingMessage),
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status:      1,
						StderrRegex: fatalErr,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							`failed to load the cloud credentials`,
							string(clitest.CloudDisablingMessage),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + regexp.QuoteMeta(string(cloud.ErrNotFound)),
							`FTL failed to load credentials`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + regexp.QuoteMeta(string(cloud.ErrNotFound)),
							`FTL failed to load credentials`,
						},
					},
				},
			},
		},
		{
			name: "/v1/memberships returns no memberships",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							`FTL ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							string(clitest.ErrCloudOnboardingIncomplete),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							`FTL ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							string(clitest.ErrCloudOnboardingIncomplete),
							string(clitest.CloudDisablingMessage),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\n",
						StderrRegexes: []string{
							`Warning: You are not part of an organization. Please visit cloud.terramate.io to create an organization.`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\n",
						StderrRegexes: []string{
							`Warning: You are not part of an organization. Please visit cloud.terramate.io to create an organization.`,
						},
					},
				},
			},
		},
		{
			name: "/v1/memberships returns multiple memberships",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[
									{
										"org_name": "terramate-io",
										"org_display_name": "Terramate",
										"org_uuid": "c7d721ee-f455-4d3c-934b-b1d96bbaad17",
										"status": "active"
									},
									{
										"org_name": "mineiros-io",
										"org_display_name": "Mineiros",
										"org_uuid": "b2f153e8-ceb1-4f26-898e-eb7789869bee",
										"status": "active"
									}
								]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`Please set TM_CLOUD_ORGANIZATION environment variable`,
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							`Please set TM_CLOUD_ORGANIZATION environment variable`,
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`Please set TM_CLOUD_ORGANIZATION environment variable`,
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							`Please set TM_CLOUD_ORGANIZATION environment variable`,
							clitest.CloudDisablingMessage,
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
			},
		},
		{
			name: "/v1/memberships returns no active memberships",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[
									{
										"org_name": "terramate-io",
										"org_display_name": "Terramate",
										"org_uuid": "c7d721ee-f455-4d3c-934b-b1d96bbaad17",
										"status": "invited"
									},
									{
										"org_name": "mineiros-io",
										"org_display_name": "Mineiros",
										"org_uuid": "b2f153e8-ceb1-4f26-898e-eb7789869bee",
										"status": "invited"
									}
								]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + clitest.CloudNoMembershipMessage,
							`WRN You have pending invitation for the following organizations: terramate-io, mineiros-io`,
							`FTL ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							`ERR ` + clitest.CloudNoMembershipMessage,
							`WRN You have pending invitation for the following organizations: terramate-io, mineiros-io`,
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`ERR ` + clitest.CloudNoMembershipMessage,
							`WRN You have pending invitation for the following organizations: terramate-io, mineiros-io`,
							`FTL ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegexes: []string{
							`ERR ` + clitest.CloudNoMembershipMessage,
							`WRN You have pending invitation for the following organizations: terramate-io, mineiros-io`,
							clitest.CloudDisablingMessage,
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
			},
		},
		{
			name: "/v1/memberships returns 1 single active memberships out of many",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[
									{
										"org_name": "terramate-io",
										"org_display_name": "Terramate",
										"org_uuid": "c7d721ee-f455-4d3c-934b-b1d96bbaad17",
										"status": "invited"
									},
									{
										"org_name": "mineiros-io",
										"org_display_name": "Mineiros",
										"org_uuid": "b2f153e8-ceb1-4f26-898e-eb7789869bee",
										"status": "active"
									}
								]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: runExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: batman\nemail: batman@example.com\norganizations: terramate-io, mineiros-io\n",
					},
				},
			},
		},
		{
			name: "/v1/deployments is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: false,
				cloud.DriftsPath:      true,
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
			},
		},
		{
			name: "/v1/deployments returns invalid payload",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: false,
				cloud.DriftsPath:      true,
			},
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"POST": {
						Path: fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath),
						Handler: http.HandlerFunc(
							func(w http.ResponseWriter, _ *http.Request) {
								writeJSON(w, `[
									{
										"stack_id": 1,
										"meta_id": "strange-meta-id"
									}
								]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
			},
		},
		{
			name: "not all stacks have ID",
			layout: []string{
				"s:stack-1:id=stack-1",
				"s:stack-2",
			},
			endpoints: testserver.EnableAllConfig(),
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`FTL ` + string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 0,
						StderrRegexes: []string{
							string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 1,
						StderrRegexes: []string{
							`FTL ` + string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status: 0,
						StderrRegexes: []string{
							string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
			},
		},
		{
			name: "/v1/drifts is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: true,
				cloud.DriftsPath:      false,
			},
			subcases: []subtestcase{
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status:      0,
						StderrRegex: clitest.CloudSyncDriftFailedMessage,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", testHelperBin, "true",
					},
					want: runExpected{
						Status:      0,
						StderrRegex: clitest.CloudSyncDriftFailedMessage,
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, subcase := range tc.subcases {
				subcase := subcase
				uimode := "human"
				if subcase.uimode == cli.AutomationMode {
					uimode = "automation"
				}
				t.Run(fmt.Sprintf("%s - %s", uimode, subcase.name), func(t *testing.T) {
					if len(subcase.cmd) == 0 {
						t.Fatal("invalid testcase: cmd not set")
					}
					env := removeEnv(os.Environ(), "CI")
					if subcase.uimode == cli.AutomationMode {
						env = append(env, "CI=true")
					}
					router := testserver.RouterWith(tc.endpoints)
					fakeserver := &http.Server{
						Handler: router,
						Addr:    "localhost:3001",
					}
					testserver.RouterAddCustoms(router, tc.customEndpoints)

					const fakeserverShutdownTimeout = 3 * time.Second
					errChan := make(chan error)
					go func() {
						errChan <- fakeserver.ListenAndServe()
					}()

					t.Cleanup(func() {
						err := fakeserver.Close()
						if err != nil {
							t.Logf("fakeserver HTTP Close error: %v", err)
						}
						select {
						case err := <-errChan:
							if err != nil && !errors.Is(err, http.ErrServerClosed) {
								t.Error(err)
							}
						case <-time.After(fakeserverShutdownTimeout):
							t.Error("time excedeed waiting for fakeserver shutdown")
						}
					})

					s := sandbox.New(t)
					layout := tc.layout
					if len(layout) == 0 {
						layout = []string{
							"s:stack:id=test",
						}
					}
					s.BuildTree(layout)
					s.Git().CommitAll("created stacks")
					tm := newCLI(t, s.RootDir(), env...)
					tm.loglevel = zerolog.WarnLevel.String()
					assertRunResult(t, tm.run(subcase.cmd...), subcase.want)
				})
			}
		})
	}
}
