// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncPreview(t *testing.T) {
	t.Parallel()
	type want struct {
		run         RunExpected
		preview     *cloudstore.Preview
		ignoreTypes cmp.Option
	}
	type testcase struct {
		name            string
		layout          []string
		skipIDGen       bool
		runflags        []string
		workingDir      string
		env             []string
		cloudData       *cloudstore.Data
		githubEventPath string
		cmd             []string
		want            want
	}

	for _, tc := range []testcase{
		{
			name: "basic success sync",
			layout: []string{
				"s:stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			runflags:        []string{`--cloud-sync-terraform-plan-file=out.tfplan`},
			cmd:             []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
			githubEventPath: datapath(t, "interop/testdata/event_pull_request.json"),
			want: want{
				run: RunExpected{
					Status: 0,
					StdoutRegexes: []string{
						"Plan: 1 to add, 0 to change, 0 to destroy.",
					},
					StderrRegexes: []string{
						"Preview created",
					},
				},
				preview: &cloudstore.Preview{
					PreviewID:       "1",
					Technology:      "terraform",
					TechnologyLayer: "default",
					UpdatedAt:       1707482312,
					StackPreviews: []cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "changed",
							Cmd:    []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
						},
					},
				},
				ignoreTypes: cmpopts.IgnoreTypes(
					cloud.CommandLogs{},
					&cloud.ChangesetDetails{},
					cloudstore.Stack{},
					&cloud.ReviewRequest{},
					&cloud.DeploymentMetadata{},
				),
			},
		},
		{
			name: "failure of command should still create preview with stack preview status failed",
			layout: []string{
				"s:stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			runflags:        []string{`--cloud-sync-terraform-plan-file=out.tfplan`},
			cmd:             []string{TerraformTestPath, "plan-invalid-subcommand", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
			githubEventPath: datapath(t, "interop/testdata/event_pull_request.json"),
			want: want{
				run: RunExpected{
					Status: 1,
					StderrRegexes: []string{
						"Preview created",
					},
				},
				preview: &cloudstore.Preview{
					PreviewID:       "1",
					Technology:      "terraform",
					TechnologyLayer: "default",
					UpdatedAt:       1707482312,
					StackPreviews: []cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "failed",
							Cmd:    []string{TerraformTestPath, "plan-invalid-subcommand", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
						},
					},
				},
				ignoreTypes: cmpopts.IgnoreTypes(
					cloud.CommandLogs{},
					&cloud.ChangesetDetails{},
					cloudstore.Stack{},
					&cloud.ReviewRequest{},
					&cloud.DeploymentMetadata{},
				),
			},
		},
	} {
		tc := tc
		name := tc.name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cloudData := tc.cloudData
			if cloudData == nil {
				var err error
				cloudData, err = cloudstore.LoadDatastore(testserverJSONFile)
				assert.NoError(t, err)
			}
			addr := startFakeTMCServer(t, cloudData)

			s := sandbox.New(t)

			// needed for invoking `terraform ...` commands in the sandbox
			s.Env, _ = test.PrependToPath(os.Environ(), filepath.Dir(TerraformTestPath))
			s.Env = append(s.Env, tc.env...)

			var genIdsLayout []string
			if !tc.skipIDGen {
				for _, layout := range tc.layout {
					if layout[0] == 's' {
						if strings.Contains(layout, "id=") {
							t.Fatalf("testcases should not contain stack IDs but found %s", layout)
						}
						id := strings.ToLower(strings.Replace(layout[2:]+"-id-"+t.Name(), "/", "-", -1))
						if len(id) > 64 {
							id = id[:64]
						}
						layout += ":id=" + id
					}
					genIdsLayout = append(genIdsLayout, layout)
				}
			} else {
				genIdsLayout = tc.layout
			}

			s.BuildTree(genIdsLayout)
			s.Git().CommitAll("all stacks committed")

			t.Logf("addr: %s", addr)
			env := RemoveEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr)
			env = append(env, "TM_GITHUB_API_URL=http://"+addr+"/")
			env = append(env, "GITHUB_EVENT_PATH="+tc.githubEventPath)
			env = append(env, tc.env...)
			cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			cli.PrependToPath(filepath.Dir(TerraformTestPath))

			s.Git().SetRemoteURL("origin", normalizedTestRemoteRepo)

			runflags := []string{
				"run",
				"--disable-safeguards=all",
				"--cloud-sync-preview",
			}
			runflags = append(runflags, tc.runflags...)
			runflags = append(runflags, "--")
			runflags = append(runflags, tc.cmd...)
			result := cli.Run(runflags...)
			AssertRunResult(t, result, tc.want.run)

			orguuid := string(cloudData.MustOrgByName("terramate").UUID)
			req, err := http.NewRequest("GET", "http://"+addr+"/v1/previews/"+orguuid+"/1", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			req.Header.Set("User-Agent", "terramate/0.0.0-test")
			httpClient := &http.Client{}
			httpResp, err := httpClient.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer func() { _ = httpResp.Body.Close() }()

			assert.EqualInts(t, httpResp.StatusCode, http.StatusOK)
			var previewResp cloudstore.Preview
			if err := json.NewDecoder(httpResp.Body).Decode(&previewResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tc.want.preview != nil {
				if diff := cmp.Diff(*(tc.want.preview), previewResp, tc.want.ignoreTypes); diff != "" {
					t.Errorf("unexpected  preview: %s", diff)
				}
			}
		})
	}
}

func datapath(t *testing.T, path string) string {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	return filepath.Join(wd, filepath.FromSlash(path))
}
