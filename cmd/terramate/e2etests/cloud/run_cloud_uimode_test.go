// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/apparentlymart/go-versions/versions"
	"github.com/julienschmidt/httprouter"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudSyncUIMode(t *testing.T) {
	t.Parallel()

	type subtestcase struct {
		name   string
		cmd    []string
		want   RunExpected
		uimode cli.UIMode
	}

	type testcase struct {
		name            string
		layout          []string
		endpoints       map[string]bool
		customEndpoints testserver.Custom
		cloudData       *cloudstore.Data
		wellknown       *cloud.WellKnown
		subcases        []subtestcase
	}

	writeJSON := func(w http.ResponseWriter, str string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(str))
	}

	const invalidUserData = `{
		"email": "batman@terramate.io",
		"display_name": "",
		"job_title": ""
	}`

	const fatalErr = `FTL ` + string(clitest.ErrCloud)

	versionNoPrerelease := versions.MustParseVersion(terramate.Version())
	versionNoPrerelease.Prerelease = ""

	for _, tc := range []testcase{
		{
			name:      "/.well-known/cli.json is not found -- everything works",
			endpoints: testserver.DisableEndpoints(cloud.WellKnownCLIPath),
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate\n",
					},
				},
			},
		},
		{
			name:      "/.well-known/cli.json returns unsupported version constraint",
			endpoints: testserver.EnableAllConfig(),
			wellknown: &cloud.WellKnown{
				RequiredVersion: "> " + versionNoPrerelease.String(),
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 0,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
							string(clitest.ErrCloud),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 0,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
							string(clitest.ErrCloud),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
						},
					},
				},
			},
		},
		{
			name:      "/.well-known/cli.json with valid constraint",
			endpoints: testserver.EnableAllConfig(),
			wellknown: &cloud.WellKnown{
				RequiredVersion: "= " + versionNoPrerelease.String(),
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate\n",
					},
				},
			},
		},
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
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
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							`FTL failed to load credentials: ` + string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
			},
		},
		{
			name:      "/v1/users returns unexpected payload",
			endpoints: testserver.DisableEndpoints(cloud.UsersPath),
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.UsersPath,
						Handler: testserver.Handler(
							func(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
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
					want: RunExpected{
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
			name:      "/v1/memberships is not working",
			endpoints: testserver.DisableEndpoints(cloud.MembershipsPath),
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegex: string(clitest.CloudDisablingMessage),
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
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
					want: RunExpected{
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
			name:      "/v1/memberships returns no memberships",
			endpoints: testserver.DisableEndpoints(cloud.MembershipsPath),
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: testserver.Handler(
							func(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\n",
						StderrRegexes: []string{
							`Warning: You are not part of an organization. Please visit cloud.terramate.io to create an organization.`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\n",
						StderrRegexes: []string{
							`Warning: You are not part of an organization. Please visit cloud.terramate.io to create an organization.`,
						},
					},
				},
			},
		},
		{
			name:      "/v1/memberships returns multiple memberships",
			endpoints: testserver.EnableAllConfig(),
			cloudData: &cloudstore.Data{
				Orgs: map[string]cloudstore.Org{
					"terramate": {
						UUID:        "deadbeef-dead-dead-dead-deaddeafbeef",
						Name:        "terramate",
						DisplayName: "Terramate",
						Domain:      "terramate.io",
						Members: []cloudstore.Member{
							{
								UserUUID: "deadbeef-dead-dead-dead-deaddeafbeef",
								Role:     "member",
								Status:   "active",
							},
						},
					},
					"mineiros": {
						UUID:        "deadbeef-dead-dead-dead-deaddeaf0001",
						Name:        "mineiros",
						DisplayName: "Mineiros",
						Domain:      "mineiros.io",
						Members: []cloudstore.Member{
							{
								UserUUID: "deadbeef-dead-dead-dead-deaddeafbeef",
								Role:     "member",
								Status:   "active",
							},
						},
					},
				},
				Users: map[string]cloud.User{
					"batman": {
						UUID:        "deadbeef-dead-dead-dead-deaddeafbeef",
						Email:       "batman@terramate.io",
						DisplayName: "Batman",
						JobTitle:    "Entrepreneur",
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: mineiros, terramate\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: mineiros, terramate\n",
					},
				},
			},
		},
		{
			name:      "/v1/memberships returns no active memberships",
			endpoints: testserver.DisableEndpoints(cloud.MembershipsPath),
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: testserver.Handler(
							func(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate-io, mineiros-io\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: terramate-io, mineiros-io\n",
					},
				},
			},
		},
		{
			name:      "/v1/memberships returns 1 single active memberships out of many",
			endpoints: testserver.EnableAllConfig(),
			cloudData: &cloudstore.Data{
				Orgs: map[string]cloudstore.Org{
					"terramate": {
						UUID:        "deadbeef-dead-dead-dead-deaddeafbeef",
						Name:        "terramate",
						DisplayName: "Terramate",
						Domain:      "terramate.io",
						Members: []cloudstore.Member{
							{
								UserUUID: "deadbeef-dead-dead-dead-deaddeafbeef",
								Role:     "member",
								Status:   "active",
							},
						},
					},
					"mineiros": {
						UUID:        "deadbeef-dead-dead-dead-deaddeaf0001",
						Name:        "mineiros",
						DisplayName: "Mineiros",
						Domain:      "mineiros.io",
						Members: []cloudstore.Member{
							{
								UserUUID: "deadbeef-dead-dead-dead-deaddeafbeef",
								Role:     "member",
								Status:   "invited",
							},
						},
					},
				},
				Users: map[string]cloud.User{
					"batman": {
						UUID:        "deadbeef-dead-dead-dead-deaddeafbeef",
						Email:       "batman@terramate.io",
						DisplayName: "Batman",
						JobTitle:    "Entrepreneur",
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},
				{
					name:   "syncing a drift",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						IgnoreStderr: true,
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: cli.HumanMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: mineiros, terramate\n",
					},
				},
				{
					name:   "cloud info",
					uimode: cli.AutomationMode,
					cmd:    []string{"experimental", "cloud", "info"},
					want: RunExpected{
						IgnoreStderr: true,
						Status:       0,
						Stdout:       "status: signed in\nprovider: Google Social Provider\nuser: Batman\nemail: batman@terramate.io\norganizations: mineiros, terramate\n",
					},
				},
			},
		},
		{
			name:      "/v1/deployments is not working",
			endpoints: testserver.DisableEndpoints(cloud.DeploymentsPath),
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: cli.HumanMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
			},
		},
		{
			name:      "/v1/deployments returns invalid payload",
			endpoints: testserver.DisableEndpoints(cloud.DeploymentsPath),
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"POST": {
						Path: fmt.Sprintf("%s/:orguuid/:deployuuid/stacks", cloud.DeploymentsPath),
						Handler: testserver.Handler(
							func(store *cloudstore.Data, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegex: clitest.CloudDisablingMessage,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: cli.AutomationMode,
					cmd: []string{
						"run",
						"--cloud-sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
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
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      0,
						StderrRegex: clitest.CloudSyncDriftFailedMessage,
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, subcase := range tc.subcases {
				subcase := subcase
				uimode := "human"
				if subcase.uimode == cli.AutomationMode {
					uimode = "automation"
				}
				t.Run(fmt.Sprintf("%s - %s", uimode, subcase.name), func(t *testing.T) {
					t.Parallel()
					if len(subcase.cmd) == 0 {
						t.Fatal("invalid testcase: cmd not set")
					}
					env := RemoveEnv(os.Environ(), "CI")
					if subcase.uimode == cli.AutomationMode {
						env = append(env, "CI=true")
					}
					listener, err := net.Listen("tcp", ":0")
					assert.NoError(t, err)
					env = append(env, "TMC_API_URL=http://"+listener.Addr().String())

					var store *cloudstore.Data
					if tc.cloudData != nil {
						// needs to be copied otherwise the sub-testcases will reuse the same store.
						// A simple copy will not work because the type embed a mutex.
						// TODO(i4k): This is a quick fix but inefficient.
						dataContent, err := tc.cloudData.MarshalJSON()
						assert.NoError(t, err)
						var data cloudstore.Data
						assert.NoError(t, json.Unmarshal(dataContent, &data))
						store = &data
					} else {
						store, err = cloudstore.LoadDatastore(testserverJSONFile)
						assert.NoError(t, err)
					}

					if tc.wellknown != nil {
						store.WellKnown = tc.wellknown
					}

					router := testserver.RouterWith(store, tc.endpoints)
					fakeserver := &http.Server{
						Handler: router,
						Addr:    listener.Addr().String(),
					}
					testserver.RouterAddCustoms(router, store, tc.customEndpoints)

					const fakeserverShutdownTimeout = 3 * time.Second
					errChan := make(chan error)
					go func() {
						errChan <- fakeserver.Serve(listener)
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
					tm := NewCLI(t, s.RootDir(), env...)
					tm.LogLevel = zerolog.WarnLevel.String()
					AssertRunResult(t, tm.Run(subcase.cmd...), subcase.want)
				})
			}
		})
	}
}
