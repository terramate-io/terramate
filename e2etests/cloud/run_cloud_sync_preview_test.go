// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

const testPreviewRemoteRepoURL = "github.com/terramate-io/dummy-repo.git"

var normalizedPreviewTestRemoteRepo string

func init() {
	normalizedPreviewTestRemoteRepo = cloud.NormalizeGitURI(testPreviewRemoteRepoURL)
}

func TestCLIRunWithCloudSyncPreview(t *testing.T) {
	t.Parallel()
	type Metadata struct {
		GithubPullRequestURL       string
		GithubPullRequestNumber    int
		GithubPullRequestTitle     string
		GithubPullRequestUpdatedAt string
		GithubPullRequestPushedAt  string
	}
	type want struct {
		run         RunExpected
		preview     *cloudstore.Preview
		Metadata    *Metadata
		ignoreTypes []cmp.Option
	}
	type testcase struct {
		name            string
		layout          []string
		runflags        []string
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
				"s:stack:id=stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			runflags:        []string{`--terraform-plan-file=out.tfplan`},
			cmd:             []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
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
				"s:stack:id=stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			runflags: []string{`--terraform-plan-file=out.tfplan`},
			cmd:      []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
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
					PushedAt:        pushedAt.Unix(),                            // pushed_at from the pull request event (not from API)
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
						Repository:  testPreviewRemoteRepoURL,
						Number:      1347,
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
		{
			name: "basic success sync with custom target",
			layout: []string{
				"s:stack:id=stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
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
			runflags: []string{`--terraform-plan-file=out.tfplan`, "--target", "custom_target"},
			cmd:      []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
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
					PushedAt:        pushedAt.Unix(),                            // pushed_at from the pull request event (not from API)
					CommitSHA:       "ea61b5bd72dec0878ae388b04d76a988439d1e28", // commit_sha from the pull request event (not from API)
					StackPreviews: []*cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "changed",
							Cmd:    []string{TerraformTestPath, "plan", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
							Stack: cloudstore.Stack{
								Stack: cloud.Stack{
									Repository:    "github.com/terramate-io/dummy-repo.git",
									Target:        "custom_target",
									DefaultBranch: "main",
									Path:          "/stack",
									MetaID:        "stack",
									MetaName:      "stack",
								},
							},
						},
					},
					ReviewRequest: &cloud.ReviewRequest{
						Platform:    "github",
						Repository:  testPreviewRemoteRepoURL,
						Number:      1347,
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
		{
			name: "failure of command should still create preview with stack preview status failed",
			layout: []string{
				"s:stack:id=stack",
				`f:stack/main.tf:
				  resource "local_file" "foo" {
					content  = "test content"
					filename = "${path.module}/foo.bar"
				  }`,
				"run:stack:terraform init",
			},
			runflags: []string{`--terraform-plan-file=out.tfplan`},
			cmd:      []string{TerraformTestPath, "plan-invalid-subcommand", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
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
					ReviewRequest: &cloud.ReviewRequest{
						Platform:    "github",
						Repository:  "github.com/terramate-io/dummy-repo.git",
						Number:      1347,
						Title:       "Amazing new feature",
						Description: "Please pull these awesome changes in!",
						URL:         "https://github.com/octocat/Hello-World/pull/1347",
						Labels:      []cloud.Label{{Name: "bug", Color: "f29513", Description: "Something isn't working"}},
						Author:      cloud.Author{Login: "octocat", AvatarURL: "https://github.com/images/error/octocat_happy.gif"},
						Status:      "open",
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
						PushedAt:    pushedAt,
						Branch:      "new-topic",
						BaseBranch:  "master",
					},
					StackPreviews: []*cloudstore.StackPreview{
						{
							ID:     "1",
							Status: "failed",
							Cmd:    []string{TerraformTestPath, "plan-invalid-subcommand", "-out=out.tfplan", "-no-color", "-detailed-exitcode"},
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

			s.Git().SetRemoteURL("origin", normalizedPreviewTestRemoteRepo)

			runflags := []string{
				"run",
				"--disable-safeguards=all",
				"--sync-preview",
			}
			runflags = append(runflags, tc.runflags...)
			runflags = append(runflags, "--")
			runflags = append(runflags, tc.cmd...)
			result := cli.Run(runflags...)
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

				assert.EqualInts(t, httpResp.StatusCode, http.StatusOK)
				if err := json.NewDecoder(httpResp.Body).Decode(&previewResp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if diff := cmp.Diff(*(tc.want.preview), previewResp, tc.want.ignoreTypes...); diff != "" {
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

func datapath(t *testing.T, path string) string {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	return filepath.Join(wd, filepath.FromSlash(path))
}

func toTime(t *testing.T, s string) *time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	assert.NoError(t, err)
	return &tm
}
