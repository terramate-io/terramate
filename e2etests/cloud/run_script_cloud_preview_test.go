// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
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
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
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
		ignoreTypes            []cmp.Option
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

	createdAt := toTime(t, "2011-01-26T19:01:12Z")
	updatedAt := toTime(t, "2011-01-26T19:01:12Z")
	pushedAt := toTime(t, "2024-02-09T12:38:30Z")

	for _, tc := range []testcase{
		{
			name: "warning when not running with GITHUB_ACTIONS",
			layout: []string{
				"s:stack:id=someid",
				`f:terramate.tm:
				  terramate {
					config {
						experiments = ["scripts"]
					}
				  }
				`,
				`f:stack/preview.tm:

				  script "preview" {
					description = "sync a preview"
					job {
					  commands = [
						["terraform", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode", {
						  sync_preview             = true,
						  terraform_plan_file = "out.tfplan",
						}],
					  ]
					}
				  }
				  `,
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
						"--sync-preview is only supported in GitHub Actions workflows",
					},
				},
				ignoreTypes: []cmp.Option{
					cmpopts.IgnoreTypes(
						cloud.CommandLogs{},
						&cloud.ChangesetDetails{},
						cloudstore.Stack{},
						&cloud.DeploymentMetadata{},
					),
					cmpopts.IgnoreFields(cloud.ReviewRequest{}, "CommitSHA"),
				},
			},
		},
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
				`f:stack/preview.tm:

				  script "preview" {
					description = "sync a preview"
					job {
					  commands = [
						["terraform", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode", {
						  sync_preview             = true,
						  terraform_plan_file = "out.tfplan",
						}],
					  ]
					}
				  }
				  `,
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			cmd: []string{"script", "run", "-X", "preview"},
			env: []string{
				"GITHUB_ACTIONS=1",
			},
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
					PushedAt:        1707482310,                                 // pushed_at from the pull request event (not from API)
					CommitSHA:       "ea61b5bd72dec0878ae388b04d76a988439d1e28", // commit_sha from the pull request event (not from API)
					StackPreviews: []*cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "changed",
							Cmd:    []string{"terraform", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
						},
					},
					ReviewRequest: &cloud.ReviewRequest{
						Platform:    "github",
						Repository:  normalizedPreviewTestRemoteRepo,
						Number:      1347,
						CommitSHA:   "aaa",
						Title:       "Amazing new feature",
						Description: "Please pull these awesome changes in!",
						URL:         "https://github.com/octocat/Hello-World/pull/1347",
						Labels:      []cloud.Label{{Name: "bug", Color: "f29513", Description: "Something isn't working"}},
						Status:      "open",
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
						PushedAt:    pushedAt,
						Author: cloud.Author{
							Login:     "octocat",
							AvatarURL: "https://github.com/images/error/octocat_happy.gif",
						},
						Branch:     "new-topic",
						BaseBranch: "master",
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
				ignoreTypes: []cmp.Option{
					cmpopts.IgnoreTypes(
						cloud.CommandLogs{},
						&cloud.ChangesetDetails{},
						cloudstore.Stack{},
						&cloud.DeploymentMetadata{},
					),
					cmpopts.IgnoreFields(cloud.ReviewRequest{}, "CommitSHA"),
				},
			},
		},
		{
			name: "failed command without sync, still sync if any other command has sync enabled",
			layout: []string{
				"s:stack:id=someid",
				`f:terramate.tm:
				  terramate {
					config {
						experiments = ["scripts"]
					}
				  }
				`,
				`f:stack/preview.tm:

				  script "preview" {
					description = "sync a preview"
					job {
					  commands = [
					    ["do-not-exist-command"],
						["terraform", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode", {
						  sync_preview             = true,
						  terraform_plan_file = "out.tfplan",
						}],
					  ]
					}
				  }
				  `,
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			cmd: []string{"script", "run", "-X", "preview"},
			env: []string{
				"GITHUB_ACTIONS=1",
			},
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
					PushedAt:        1707482310,                                 // pushed_at from the pull request event (not from API)
					CommitSHA:       "ea61b5bd72dec0878ae388b04d76a988439d1e28", // commit_sha from the pull request event (not from API)
					StackPreviews: []*cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "failed",
							Cmd:    []string{"terraform", "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
						},
					},
					ReviewRequest: &cloud.ReviewRequest{
						Platform:    "github",
						Repository:  normalizedPreviewTestRemoteRepo,
						Number:      1347,
						CommitSHA:   "aaa",
						Title:       "Amazing new feature",
						Description: "Please pull these awesome changes in!",
						URL:         "https://github.com/octocat/Hello-World/pull/1347",
						Labels:      []cloud.Label{{Name: "bug", Color: "f29513", Description: "Something isn't working"}},
						Status:      "open",
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
						PushedAt:    pushedAt,
						Author: cloud.Author{
							Login:     "octocat",
							AvatarURL: "https://github.com/images/error/octocat_happy.gif",
						},
						Branch:     "new-topic",
						BaseBranch: "master",
					},
				},
				Metadata: &Metadata{
					GithubPullRequestURL:       "https://github.com/octocat/Hello-World/pull/1347",
					GithubPullRequestNumber:    1347,
					GithubPullRequestTitle:     "Amazing new feature",
					GithubPullRequestUpdatedAt: "2011-01-26T19:01:12Z",
				},
				ignoreTypes: []cmp.Option{
					cmpopts.IgnoreTypes(
						cloud.CommandLogs{},
						&cloud.ChangesetDetails{},
						cloudstore.Stack{},
						&cloud.DeploymentMetadata{},
					),
					cmpopts.IgnoreFields(cloud.ReviewRequest{}, "CommitSHA"),
				},
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
			env = RemoveEnv(env, "GITHUB_ACTIONS")
			env = append(env, "TMC_API_URL=http://"+addr)
			env = append(env, "TM_GITHUB_API_URL=http://"+addr+"/")
			env = append(env, "GITHUB_EVENT_PATH="+tc.githubEventPath)
			env = append(env, "GITHUB_TOKEN=fake_token")
			env = append(env, tc.env...)
			cli := NewCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)), env...)
			cli.PrependToPath(filepath.Dir(TerraformTestPath))

			s.Git().SetRemoteURL("origin", testPreviewRemoteRepoURL)

			result := cli.Run(tc.cmd...)
			AssertRunResult(t, result, tc.want.run)

			var previewResp cloudstore.Preview
			if tc.want.preview != nil {
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

				assert.EqualInts(t, http.StatusOK, httpResp.StatusCode)
				if err := json.NewDecoder(httpResp.Body).Decode(&previewResp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if diff := cmp.Diff(*(tc.want.preview), previewResp, tc.want.ignoreTypes...); diff != "" {
					t.Errorf("unexpected  preview: %s", diff)
				}
			}

			if tc.want.stackPreviewChangesets != nil {
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
