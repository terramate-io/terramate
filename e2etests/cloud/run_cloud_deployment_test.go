// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cli/safeexec"
	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/api/resources"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/ui/tui/clitest"
)

const testRemoteRepoURL = "https://terramate.io/terramate-io/terramate.git"

var normalizedTestRemoteRepo string

func init() {
	r, err := git.NormalizeGitURI(testRemoteRepoURL)
	if err != nil {
		panic(err)
	}
	normalizedTestRemoteRepo = r.Repo
	if normalizedTestRemoteRepo == "" {
		panic("empty normalized repo")
	}
}

func TestCLIRunWithCloudSyncDeployment(t *testing.T) {
	t.Parallel()

	type want struct {
		run    RunExpected
		events eventsResponse
	}
	type testcase struct {
		name       string
		layout     []string
		runflags   []string
		workingDir string
		env        []string
		cloudData  *cloudstore.Data
		cmd        []string
		want       want
	}

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			cmd: []string{HelperPath, "echo", "ok"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: string(clitest.ErrCloudStacksWithoutID),
				},
			},
		},
		{
			name:   "failed command",
			layout: []string{"s:stack:id=stack"},
			cmd:    []string{"non-existent-command"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"stack": []string{"pending", "failed"},
				},
			},
		},
		{
			name:   "command not found cancels execution of subsequent stacks",
			layout: []string{"s:s1:id=s1", "s:s1/s2:id=s2"},
			cmd:    []string{"non-existent-command"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"s1": []string{"pending", "failed"},
					"s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name:   "failed cmd cancels execution of subsequent stacks",
			layout: []string{"s:s1:id=s1", "s:s1/s2:id=s2"},
			cmd:    []string{HelperPath, "exit", "1"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "one or more commands failed",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name:     "both failed stacks and continueOnError",
			layout:   []string{"s:s1:id=s1", "s:s2:id=s2"},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{"non-existent-command"},
			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"s1": []string{"pending", "failed"},
					"s2": []string{"pending", "failed"},
				},
			},
		},
		{
			name: "failed cmd and continueOnError",
			layout: []string{
				"s:s1:id=s1",
				"s:s1/s2:id=s2",
				"f:s1/s2/test.txt:test",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{HelperPath, "cat", "test.txt"},
			want: want{
				run: RunExpected{
					Status: 1,
					Stdout: "test",
					StderrRegexes: []string{
						"Error: one or more commands failed",
						"execution failed",
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:     "basic success sync",
			layout:   []string{"s:stack:id=stack"},
			runflags: []string{`--eval`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			want: want{
				run: RunExpected{
					Stdout: "/stack\n",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:     "basic success sync - mixed case stack ID",
			layout:   []string{"s:stack:id=StAcK"},
			runflags: []string{`--eval`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			want: want{
				run: RunExpected{
					Stdout: "/stack\n",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "outputs sharing success - string",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				`s:s2:id=s2;after=["/s1"]`,
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
					Str("mock", "test"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
			},
			cmd: []string{"terraform", "apply", "-auto-approve"},
			want: want{
				run: RunExpected{
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
						regexp.QuoteMeta(`local_file.s2_file`),
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "outputs sharing success - object",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				`s:s2:id=s2;after=["/s1"]`,
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", `outputs.s1_output`),
					Str("from_stack_id", "s1"),
					Str("mock", "test"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input.value"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
			},
			cmd: []string{"terraform", "apply", "-auto-approve"},
			want: want{
				run: RunExpected{
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
						regexp.QuoteMeta(`local_file.s2_file`),
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "outputs sharing success - list",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				`s:s2:id=s2;after=["/s1"]`,
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "[0, 1, 2, outputs.s1_output.value, 4]"),
					Str("from_stack_id", "s1"),
					Str("mock", "test"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input[3]"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
			},
			cmd: []string{"terraform", "apply", "-auto-approve"},
			want: want{
				run: RunExpected{
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
						regexp.QuoteMeta(`local_file.s2_file`),
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "outputs sharing sync failed result",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				"s:s2:id=s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
				"--mock-on-fail",
			},
			cmd: []string{"terraform", "plan", "-out=out.tfplan"},
			want: want{
				run: RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
					},
					StderrRegex: regexp.QuoteMeta(`evaluating input value: This object does not have an attribute named`),
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "failed"},
				},
			},
		},
		{
			name: "outputs sharing sync failed when input value fails and no mock is set",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				"s:s2:id=s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
				"--mock-on-fail",
			},
			cmd: []string{"terraform", "plan", "-out=out.tfplan"},
			want: want{
				run: RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
					},
					StderrRegexes: []string{
						regexp.QuoteMeta(`evaluating input value: This object does not have an attribute named`),
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "failed"},
				},
			},
		},
		{
			name: "outputs sharing sync failed when mock cant be evaluated",
			layout: []string{
				"f:exp.tm:" + Terramate(
					Config(
						Experiments("outputs-sharing"),
					),
				).String(),
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				"s:s2:id=s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
					Expr("mock", "not.existent"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			runflags: []string{
				"--enable-sharing",
				"--mock-on-fail",
			},
			cmd: []string{"terraform", "plan", "-out=out.tfplan"},
			want: want{
				run: RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						regexp.QuoteMeta(`Terraform will perform the following actions`),
						regexp.QuoteMeta(`local_file.s1_file`),
					},
					StderrRegexes: []string{
						regexp.QuoteMeta(`evaluating input value: This object does not have an attribute named`),
						regexp.QuoteMeta(`failed to evaluate input mock`),
					},
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "failed"},
				},
			},
		},
		{
			name:     "setting TM_CLOUD_ORGANIZATION",
			layout:   []string{"s:stack:id=stack"},
			runflags: []string{`--eval`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			env:      []string{"TM_CLOUD_ORGANIZATION=terramate"},
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
			want: want{
				run: RunExpected{
					Stdout: "/stack\n",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:     "organization is case insensitive",
			layout:   []string{"s:stack:id=stack"},
			runflags: []string{`--eval`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			env:      []string{"TM_CLOUD_ORGANIZATION=TerraMate"},
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
			want: want{
				run: RunExpected{
					Status: 0,
					Stdout: "/stack\n",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "only stacks inside working dir are synced",
			layout: []string{
				"s:parent:id=parent",
				"s:parent/child:id=child",
			},
			workingDir: "parent/child",
			runflags:   []string{`--eval`},
			cmd:        []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			want: want{
				run: RunExpected{
					Status: 0,
					Stdout: "/parent/child\n",
				},
				events: eventsResponse{
					"child": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "multiple stacks",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			runflags: []string{`--eval`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},

			want: want{
				run: RunExpected{
					Status: 0,
					Stdout: "/s1\n/s2\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "target without targets enabled fails",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
			},
			runflags: []string{`--eval`, "--target", "custom-target"},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},

			want: want{
				run: RunExpected{
					Status:      1,
					StderrRegex: clitest.CloudTargetsNotEnabledMessage,
				},
			},
		},
		{
			name: "multiple stacks with custom target",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				`f:cfg.tm.hcl:terramate {
					config {
						experiments = ["targets"]
						cloud {
							targets {
								enabled = true
							}
						}
					}
				}`,
			},
			runflags: []string{`--eval`, "--target", "custom-target"},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},

			want: want{
				run: RunExpected{
					Status: 0,
					Stdout: "/s1\n/s2\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:     "skip missing plan",
			layout:   []string{"s:stack:id=stack"},
			runflags: []string{`--eval`, `--terraform-plan-file=out.tfplan`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			want: want{
				run: RunExpected{
					Stdout: "/stack\n",
					StderrRegexes: []string{
						clitest.CloudSkippingTerraformPlanSync,
					},
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "multiple stacks with plans",
			layout: []string{
				"s:s1:id=s1",
				"s:s2:id=s2",
				"copy:s1:testdata/cloud-sync-drift-plan-file",
				"copy:s2:testdata/cloud-sync-drift-plan-file",
				"run:s1:terraform init",
				"run:s1:terraform plan -no-color -out=out.tfplan",
				"run:s2:terraform init",
				"run:s2:terraform plan -no-color -out=out.tfplan",
			},
			runflags: []string{`--eval`, `--terraform-plan-file=out.tfplan`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			env: []string{
				`TF_VAR_content=my secret`,
			},
			want: want{
				run: RunExpected{
					Stdout: "/s1\n/s2\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "flag --terragrunt exports environment variables that control stdout.",
			layout: []string{
				"s:s1:id=s1",
				"copy:s1:testdata/cloud-sync-drift-plan-file",
			},
			runflags: []string{`--terragrunt`, `--eval`, `--terraform-plan-file=out.tfplan`},
			cmd:      []string{HelperPathAsHCL, "echo", "${terramate.stack.path.absolute}"},
			env: []string{
				`TF_VAR_content=my secret`,
			},
			want: want{
				run: RunExpected{
					// NOTE(i4k): Not the ideal way of testing this but the test "terragrunt" command exits with 1
					// if the required environment variables are not set.
					Status: 0,
					Stdout: nljoin("/s1"),
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
				},
			},
		},
	} {
		for _, isParallel := range []bool{false, true} {
			tc := tc
			isParallel := isParallel
			name := tc.name
			if isParallel {
				name += "-parallel"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				cloudData := tc.cloudData
				defaultOrg := "terramate"
				if cloudData == nil {
					var err error
					cloudData, defaultOrg, err = cloudstore.LoadDatastore(testserverJSONFile)
					assert.NoError(t, err)
				}
				addr := startFakeTMCServer(t, cloudData)

				s := sandbox.New(t)

				// needed for invoking `terraform ...` commands in the sandbox
				s.Env, _ = test.PrependToPath(os.Environ(), filepath.Dir(TerraformTestPath))
				s.Env = append(s.Env, tc.env...)

				s.BuildTree(tc.layout)
				env := RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS")
				env = append(env, "TMC_API_URL=http://"+addr)
				env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)
				env = append(env, tc.env...)
				env, ok := test.PrependToPath(env, filepath.Dir(TerraformTestPath))
				assert.IsTrue(t, ok)
				env, ok = test.PrependToPath(env, filepath.Dir(HelperPath)) // so our test "terragrunt" can be found.
				assert.IsTrue(t, ok)

				if slices.Contains(tc.runflags, "--enable-sharing") {
					s.Generate()
					cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
					AssertRunResult(t, cli.Run("run", "-X", "--quiet", "--", "terraform", "init"), RunExpected{
						Status:       0,
						IgnoreStdout: true,
					})
				}

				s.Git().CommitAll("all stacks committed")

				cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
				s.Git().SetRemoteURL("origin", testRemoteRepoURL)

				runflags := []string{
					"run",
					"--disable-safeguards=git-out-of-sync",
					"--quiet",
					"--sync-deployment",
				}
				if isParallel {
					runflags = append(runflags, "--parallel", "5")
					tc.want.run.IgnoreStdout = true
					tc.want.run.IgnoreStderr = true
				}
				runflags = append(runflags, tc.runflags...)
				runflags = append(runflags, "--")
				runflags = append(runflags, tc.cmd...)
				result := cli.Run(runflags...)
				AssertRunResult(t, result, tc.want.run)
				assertRunEvents(t, cloudData, s.Git().RevParse("HEAD"), tc.want.events)
			})
		}
	}
}

func assertRunEvents(t *testing.T, cloudData *cloudstore.Data, commitSHA string, expectedEvents eventsResponse) {
	if expectedEvents == nil {
		expectedEvents = make(map[string][]string)
	}

	org := cloudData.MustOrgByName("terramate")
	deployment, ok := cloudData.FindDeploymentForCommit(org.UUID, commitSHA)
	if !ok {
		if len(expectedEvents) == 0 {
			return
		}
		t.Fatalf("deployment not found but expected events: %v", expectedEvents)
	}
	cloudEvents, err := cloudData.GetDeploymentEvents(org.UUID, deployment.UUID)
	if err != nil && !errors.IsKind(err, cloudstore.ErrNotExists) {
		t.Fatal(err)
	}

	gotEvents := eventsResponse{}
	for mangled, events := range cloudEvents {
		parts := strings.Split(mangled, "|")
		target := parts[0]
		metaid := parts[1]
		st, _, found := cloudData.GetStackByMetaID(org, metaid, target)
		if !found {
			t.Fatal("stack not found")
		}
		gotEvents[st.MetaID] = []string{}
		for _, status := range events {
			gotEvents[st.MetaID] = append(gotEvents[st.MetaID], status.String())
		}
	}

	if diff := cmp.Diff(gotEvents, expectedEvents); diff != "" {
		t.Logf("want: %+v", expectedEvents)
		t.Logf("got: %+v", gotEvents)
		t.Fatal(diff)
	}
}

func TestRunGithubTokenDetection(t *testing.T) {
	t.Parallel()
	s := sandbox.New(t)
	git := s.Git()
	git.SetRemoteURL("origin", "https://github.com/any-org/any-repo")

	s.BuildTree([]string{
		"s:s1:id=s1",
		"s:s2:id=s2",
	})

	git.CommitAll("all files")

	l, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)

	store, defaultOrg, err := cloudstore.LoadDatastore(testserverJSONFile)
	assert.NoError(t, err)

	fakeserver := &http.Server{
		Handler: testserver.Router(store),
		Addr:    l.Addr().String(),
	}

	const fakeserverShutdownTimeout = 3 * time.Second
	errChan := make(chan error)
	go func() {
		errChan <- fakeserver.Serve(l)
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

	env := RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS", "GITHUB_TOKEN")
	env = append(env, "TM_CLOUD_ORGANIZATION="+defaultOrg)

	t.Run("GH_TOKEN detection", func(t *testing.T) {
		t.Parallel()
		tm := NewCLI(t, s.RootDir(), env...)
		tm.LogLevel = "debug"
		tm.AppendEnv = append(tm.AppendEnv, "GH_TOKEN=abcd")
		tm.AppendEnv = append(tm.AppendEnv, "TMC_API_URL=http://"+l.Addr().String())

		result := tm.Run("run",
			"--disable-check-git-remote",
			"--sync-deployment", "--", HelperPath, "true")
		AssertRunResult(t, result, RunExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from GH_TOKEN",
		})
	})

	t.Run("GITHUB_TOKEN detection", func(t *testing.T) {
		t.Parallel()
		tm := NewCLI(t, s.RootDir(), env...)
		tm.AppendEnv = append(tm.AppendEnv, "GITHUB_TOKEN=abcd")
		tm.AppendEnv = append(tm.AppendEnv, "TMC_API_URL=http://"+l.Addr().String())
		tm.LogLevel = "debug"

		result := tm.Run("run",
			"--disable-check-git-remote",
			"--sync-deployment", "--", HelperPath, "true")
		AssertRunResult(t, result, RunExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from GITHUB_TOKEN",
		})
	})

	t.Run("GH config file detection", func(t *testing.T) {
		t.Parallel()
		_, err := safeexec.LookPath("gh")
		if err != nil {
			t.Skip("gh tool not installed")
		}

		tm := NewCLI(t, s.RootDir(), env...)
		tm.AppendEnv = append(tm.AppendEnv, "TMC_API_URL=http://"+l.Addr().String())
		tm.LogLevel = "debug"
		ghConfigDir := test.TempDir(t)
		test.WriteFile(t, ghConfigDir, "hosts.yml", `github.com:
    user: test
    oauth_token: abcd
    git_protocol: ssh
`)
		tm.AppendEnv = append(tm.AppendEnv, "GH_CONFIG_DIR="+ghConfigDir)

		result := tm.Run("run",
			"--disable-check-git-remote",
			"--sync-deployment", "--", HelperPath, "true")
		AssertRunResult(t, result, RunExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from oauth_token",
		})
	})
}

type credential struct{}

func (c *credential) ApplyCredentials(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer abcd")
	return nil
}

func (c *credential) RedactCredentials(req *http.Request) {
	req.Header.Set("Authorization", "Bearer REDACTED")
}

type eventsResponse map[string][]string

func (res eventsResponse) Validate() error {
	return nil
}
