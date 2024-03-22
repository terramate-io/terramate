// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptRunWithCloudSyncPreview(t *testing.T) {
	t.Parallel()
	type Metadata struct {
		GithubPullRequestURL       string
		GithubPullRequestNumber    int
		GithubPullRequestTitle     string
		GithubPullRequestUpdatedAt string
		GithubPullRequestPushedAt  string
	}

	type changesetDetails struct {
		Provisioner           string
		ChangesetASCIIRegexes []string
	}

	type want struct {
		run                    RunExpected
		preview                *cloudstore.Preview
		Metadata               *Metadata
		stackPreviewChangesets []changesetDetails
		ignoreTypes            cmp.Option
	}
	type testcase struct {
		name            string
		layout          []string
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
				"s:stack:id=someid",
				`f:terramate.tm:
				  terramate {
					config {
						experiments = ["scripts"]
					}
				  }
				`,
				fmt.Sprintf(`f:stack/preview.tm:

				  script "preview" {
					description = "sync a preview"
					job {
					  commands = [
						["%s", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode", {
						  cloud_sync_preview             = true,
						  cloud_sync_terraform_plan_file = "out.tfplan",
						}],
					  ]
					}
				  }
				  `,
					TerraformTestPath,
				),
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			cmd:             []string{"script", "run", "-X", "preview"},
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
					PushedAt:        1707482310,                                 // pushed_at from the pull request event (not from API)
					CommitSHA:       "ea61b5bd72dec0878ae388b04d76a988439d1e28", // commit_sha from the pull request event (not from API)
					StackPreviews: []*cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "changed",
							Cmd:    []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
						},
					},
					ReviewRequest: &cloud.ReviewRequest{
						Platform:    "github",
						Repository:  "terramate.io/terramate-io/dummy-repo.git",
						CommitSHA:   "6dcb09b5b57875f334f61aebed695e2e4193db5e",
						Number:      1347,
						Title:       "Amazing new feature",
						Description: "Please pull these awesome changes in!",
						URL:         "https://github.com/octocat/Hello-World/pull/1347",
						Labels:      []cloud.Label{{Name: "bug", Color: "f29513", Description: "Something isn't working"}},
						Status:      "open",
						UpdatedAt:   toTime("2011-01-26T19:01:12Z"),
						PushedAt:    toTime("2024-02-09T12:38:30Z"),
					},
				},
				Metadata: &Metadata{
					GithubPullRequestURL:       "https://github.com/octocat/Hello-World/pull/1347",
					GithubPullRequestNumber:    1347,
					GithubPullRequestTitle:     "Amazing new feature",
					GithubPullRequestUpdatedAt: "2011-01-26T19:01:12Z",
				},
				stackPreviewChangesets: []changesetDetails{
					{
						Provisioner: "terraform",
						ChangesetASCIIRegexes: []string{
							`Terraform will perform the following actions:`,
							`# local_file.foo will be created`,
							`Plan: 1 to add, 0 to change, 0 to destroy`,
						},
					},
				},
				ignoreTypes: cmpopts.IgnoreTypes(
					cloud.CommandLogs{},
					&cloud.ChangesetDetails{},
					cloudstore.Stack{},
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

			s.BuildTree(tc.layout)
			s.Git().CommitAll("all stacks committed")

			env := RemoveEnv(os.Environ(), "CI")
			env = append(env, "TMC_API_URL=http://"+addr)
			env = append(env, "TM_GITHUB_API_URL=http://"+addr+"/")
			env = append(env, "GITHUB_EVENT_PATH="+tc.githubEventPath)
			env = append(env, tc.env...)
			cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			cli.PrependToPath(filepath.Dir(TerraformTestPath))

			s.Git().SetRemoteURL("origin", normalizedTestRemoteRepo)

			result := cli.Run(tc.cmd...)
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

			for i, sp := range previewResp.StackPreviews {
				assert.EqualStrings(t, tc.want.stackPreviewChangesets[i].Provisioner, sp.ChangesetDetails.Provisioner)
				for _, asciiRegex := range tc.want.stackPreviewChangesets[i].ChangesetASCIIRegexes {
					matched, err := regexp.MatchString(asciiRegex, sp.ChangesetDetails.ChangesetASCII)
					assert.NoError(t, err, "failed to compile regex %q", asciiRegex)

					if !matched {
						t.Errorf("ChangesetASCII=\"%s\" does not match regex %q",
							sp.ChangesetDetails.ChangesetASCII,
							asciiRegex,
						)
					}
				}
			}

			if tc.want.preview != nil {
				if diff := cmp.Diff(*(tc.want.preview), previewResp, tc.want.ignoreTypes); diff != "" {
					t.Errorf("unexpected  preview: %s", diff)
				}
			}

			if tc.want.Metadata != nil {
				assert.EqualStrings(t, tc.want.Metadata.GithubPullRequestURL, previewResp.Metadata.GithubPullRequestURL)
				assert.EqualInts(t, tc.want.Metadata.GithubPullRequestNumber, previewResp.Metadata.GithubPullRequestNumber)
				assert.EqualStrings(t, tc.want.Metadata.GithubPullRequestTitle, previewResp.Metadata.GithubPullRequestTitle)
				assert.EqualStrings(t, tc.want.Metadata.GithubPullRequestUpdatedAt, previewResp.Metadata.GithubPullRequestUpdatedAt.Format(time.RFC3339))
			}
		})
	}
}
