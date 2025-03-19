// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	stdhttp "net/http"
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
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/http"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

func TestCloudSyncUIMode(t *testing.T) {
	t.Parallel()

	type subtestcase struct {
		name   string
		cmd    []string
		want   RunExpected
		uimode engine.UIMode

		env []string
	}

	type testcase struct {
		name            string
		layout          []string
		endpoints       map[string]bool
		customEndpoints testserver.Custom
		cloudData       *cloudstore.Data
		wellknown       *resources.WellKnown
		subcases        []subtestcase
	}

	writeJSON := func(w stdhttp.ResponseWriter, str string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(str))
	}

	const invalidUserData = `{
		"email": "batman@terramate.io",
		"display_name": "",
		"job_title": ""
	}`

	const fatalErr = string(clitest.ErrCloud)

	versionNoPrerelease := versions.MustParseVersion(terramate.Version())
	versionNoPrerelease.Prerelease = ""

	_, defaultTestOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
	assert.NoError(t, err)

	for _, tc := range []testcase{
		{
			name:      "/.well-known/cli.json is not found -- everything works",
			endpoints: testserver.DisableEndpoints(cloud.WellKnownCLIPath),
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"selected organization: Terramate (terramate)",
						),
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"selected organization: Terramate (terramate)",
						),
					},
				},
			},
		},
		{
			name:      "/.well-known/cli.json returns unsupported version constraint",
			endpoints: testserver.EnableAllConfig(),
			wellknown: &resources.WellKnown{
				RequiredVersion: "> " + versionNoPrerelease.String(),
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
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
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
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
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
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
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudCompat),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
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
			wellknown: &resources.WellKnown{
				RequiredVersion: "= " + versionNoPrerelease.String(),
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"selected organization: Terramate (terramate)",
						),
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"selected organization: Terramate (terramate)",
						),
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.CloudOnboardingIncompleteMessage),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							string(clitest.CloudOnboardingIncompleteMessage),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.CloudOnboardingIncompleteMessage),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							string(clitest.CloudOnboardingIncompleteMessage),
							string(clitest.CloudDisablingMessage),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudOnboardingIncompleteMessage,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudOnboardingIncompleteMessage,
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
							func(_ *cloudstore.Data, w stdhttp.ResponseWriter, _ *stdhttp.Request, _ httprouter.Params) {
								writeJSON(w, invalidUserData)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							fatalErr,
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							clitest.CloudDisablingMessage,
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							`failed to load the cloud credentials`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(http.ErrUnexpectedResponseBody),
							`failed to load the cloud credentials`,
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: fatalErr,
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegex: string(clitest.CloudDisablingMessage),
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: fatalErr,
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
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
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							regexp.QuoteMeta(string(http.ErrNotFound)),
							`failed to load the cloud credentials`,
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							regexp.QuoteMeta(string(http.ErrNotFound)),
							`failed to load the cloud credentials`,
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
							func(_ *cloudstore.Data, w stdhttp.ResponseWriter, _ *stdhttp.Request, _ httprouter.Params) {
								writeJSON(w, `[]`)
							},
						),
					},
				},
			},
			subcases: []subtestcase{
				{
					name:   "syncing a deployment",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							clitest.CloudNoMembershipMessage,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
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
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
						},
					},
				},
			},
		},
		{
			name:      "check membership messages uses correct regional URLs",
			endpoints: testserver.DisableEndpoints(cloud.MembershipsPath),
			customEndpoints: testserver.Custom{
				Routes: map[string]testserver.Route{
					"GET": {
						Path: cloud.MembershipsPath,
						Handler: testserver.Handler(
							func(_ *cloudstore.Data, w stdhttp.ResponseWriter, _ *stdhttp.Request, _ httprouter.Params) {
								writeJSON(w, `[]`)
							},
						),
					},
				},
			},
			layout: []string{
				"s:stack:id=test",
				"f:region.tm:" + Terramate(
					Config(
						Block("cloud",
							Str("location", "us"),
						),
					),
				).String(),
			},
			subcases: []subtestcase{
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
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
				Users: map[string]resources.User{
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
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
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
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
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
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
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Mineiros (mineiros), Terramate (terramate)",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable or terramate.config.cloud`),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Mineiros (mineiros), Terramate (terramate)",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable or terramate.config.cloud`),
						},
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
							func(_ *cloudstore.Data, w stdhttp.ResponseWriter, _ *stdhttp.Request, _ httprouter.Params) {
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
									},
									{
										"org_name": "terramate-sso",
										"org_display_name": "Terramate SSO",
										"org_uuid": "b1f153e8-ceb1-4f26-898e-eb7789869bee",
										"status": "sso_invited"
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
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							`Error: ` + clitest.CloudNoMembershipMessage,
							`You have pending invitation for the organization terramate-io`,
							`You have pending invitation for the organization mineiros-io`,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							`Error: ` + clitest.CloudNoMembershipMessage,
							`You have pending invitation for the organization terramate-io`,
							`You have pending invitation for the organization mineiros-io`,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							`Error: ` + clitest.CloudNoMembershipMessage,
							`You have pending invitation for the organization terramate-io`,
							`You have pending invitation for the organization mineiros-io`,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							`Error: ` + clitest.CloudNoMembershipMessage,
							`You have pending invitation for the organization terramate-io`,
							`You have pending invitation for the organization mineiros-io`,
							string(clitest.ErrCloudOnboardingIncomplete),
						},
					},
				},

				// cloud info cases
				{
					name:   "cloud info",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"pending invitations: 2",
							"pending SSO invitations: 1",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
						},
					},
				},
				{
					name:   "cloud info",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"pending invitations: 2",
							"pending SSO invitations: 1",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`You are not part of an organization`),
						},
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
				Users: map[string]resources.User{
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
					name:   "syncing a deployment without setting the org",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
					},
				},
				{
					name:   "syncing a deployment without setting the org",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      0,
						StderrRegex: regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
					},
				},
				{
					name:   "syncing a deployment with org set",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
				},
				{
					name:   "syncing a deployment with org set",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					env: []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
				},
				{
					name:   "syncing a drift without org set",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
					},
				},
				{
					name:   "syncing a drift without org set",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      0,
						StderrRegex: regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
					},
				},
				{
					name:   "syncing a drift with org set",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "syncing a drift with org set",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
				},

				// cloud info cases
				{
					name:   "cloud info without an org set",
					uimode: engine.HumanMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"pending invitations: 1",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
						},
					},
				},
				{
					name:   "cloud info without an org set",
					uimode: engine.AutomationMode,
					cmd:    []string{"cloud", "info"},
					want: RunExpected{
						Status: 0,
						Stdout: nljoin(
							"provider: Google",
							"status: signed in",
							"user: Batman",
							"email: batman@terramate.io",
							"active organizations: Terramate (terramate)",
							"pending invitations: 1",
						),
						StderrRegexes: []string{
							regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
						},
					},
				},
			},
		},
		{
			name:      "/v1/deployments with",
			endpoints: testserver.EnableAllConfig(),
			subcases: []subtestcase{
				{
					name:   "org unset",
					uimode: engine.HumanMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
					},
				},
				{
					name:   "org set",
					uimode: engine.AutomationMode,
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						StderrRegexes: []string{
							regexp.QuoteMeta(`Please set TM_CLOUD_ORGANIZATION environment variable`),
							clitest.CloudDisablingMessage,
						},
					},
				},
				{
					name:   "org set",
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
				},
				{
					name:   "org unset",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--quiet",
						"--sync-deployment",
						"--", HelperPath, "true",
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: regexp.QuoteMeta("failed to create cloud deployment"),
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
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
							func(_ *cloudstore.Data, w stdhttp.ResponseWriter, _ *stdhttp.Request, _ httprouter.Params) {
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      1,
						StderrRegex: regexp.QuoteMeta(`unexpected API response body: invalid deployment status: unrecognized (0)`),
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
				{
					name:   "syncing a deployment",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-deployment",
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status: 1,
						StderrRegexes: []string{
							string(clitest.ErrCloudStacksWithoutID),
						},
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
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
					uimode: engine.HumanMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
						"--", HelperPath, "true",
					},
					want: RunExpected{
						Status:      0,
						StderrRegex: clitest.CloudSyncDriftFailedMessage,
					},
				},
				{
					name:   "syncing a drift",
					uimode: engine.AutomationMode,
					env:    []string{"TM_CLOUD_ORGANIZATION=" + defaultTestOrg},
					cmd: []string{
						"run",
						"--sync-drift-status",
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
				if subcase.uimode == engine.AutomationMode {
					uimode = "automation"
				}
				t.Run(fmt.Sprintf("%s - %s", uimode, subcase.name), func(t *testing.T) {
					t.Parallel()
					if len(subcase.cmd) == 0 {
						t.Fatal("invalid testcase: cmd not set")
					}
					env := RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS")
					if subcase.uimode == engine.AutomationMode {
						env = append(env, "CI=true")
					}
					listener, err := net.Listen("tcp", ":0")
					assert.NoError(t, err)
					env = append(env, "TMC_API_URL=http://"+listener.Addr().String())
					env = append(env, subcase.env...)

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
						store, _, err = cloudstore.LoadDatastore(testserverJSONFile)
						assert.NoError(t, err)
					}

					if tc.wellknown != nil {
						store.WellKnown = tc.wellknown
					}

					router := testserver.RouterWith(store, tc.endpoints)
					fakeserver := &stdhttp.Server{
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
							if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
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
					if subcase.cmd[0] == "run" {
						// allows for testing the metadata collection.
						s.Git().SetRemoteURL("origin", testRemoteRepoURL)
						cmd := []string{"run", "--disable-safeguards=git-out-of-sync"}
						cmd = append(cmd, subcase.cmd[1:]...)
						subcase.cmd = cmd
					}
					tm := NewCLI(t, s.RootDir(), env...)
					tm.LogLevel = zerolog.WarnLevel.String()
					AssertRunResult(t, tm.Run(subcase.cmd...), subcase.want)
				})
			}
		})
	}
}
