// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cli/safeexec"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cmd/terramate/cli"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCLIRunWithCloudSyncDeployment(t *testing.T) {
	type want struct {
		run    runExpected
		events eventsResponse
	}
	type testcase struct {
		name       string
		layout     []string
		runflags   []string
		skipIDGen  bool
		workingDir string
		cmd        []string
		want       want
		runMode    runMode
	}

	startFakeTMCServer(t)

	for _, tc := range []testcase{
		{
			name: "all stacks must have ids",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			skipIDGen: true,
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "flag requires that selected stacks contain an ID field",
				},
			},
		},
		{
			name:   "failed command",
			layout: []string{"s:stack"},
			cmd:    []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "failed"},
				},
			},
		},
		{
			name:   "failed cmd cancels execution of subsequent stacks",
			layout: []string{"s:s1", "s:s2"},
			cmd:    []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name:     "both failed stacks and continueOnError",
			layout:   []string{"s:s1", "s:s2"},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{"non-existent-command"},
			want: want{
				run: runExpected{
					Status:      1,
					StderrRegex: "executable file not found",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "running", "failed"},
				},
			},
		},
		{
			name: "failed cmd and continueOnError",
			layout: []string{
				"s:s1",
				"s:s2",
				"f:s2/test.txt:test",
			},
			runflags: []string{"--continue-on-error"},
			cmd:      []string{testHelperBin, "cat", "test.txt"},
			want: want{
				run: runExpected{
					Status:       1,
					Stdout:       "test",
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name:    "canceled hang command",
			layout:  []string{"s:stack"},
			runMode: hangRun,
			want: want{
				run: runExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"stack": []string{"pending", "running", "canceled"},
				},
			},
		},
		{
			name:    "canceled subsequent stacks",
			layout:  []string{"s:s1", "s:s2"},
			runMode: sleepRun,
			want: want{
				run: runExpected{
					Status:       1,
					IgnoreStdout: true,
					IgnoreStderr: true,
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "failed"},
					"s2": []string{"pending", "canceled"},
				},
			},
		},
		{
			name:   "basic success sync",
			layout: []string{"s:stack"},
			want: want{
				run: runExpected{
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
				"s:parent",
				"s:parent/child",
			},
			workingDir: "parent/child",
			want: want{
				run: runExpected{
					Status: 0,
					Stdout: "/parent/child\n",
				},
				events: eventsResponse{
					"parent/child": []string{"pending", "running", "ok"},
				},
			},
		},
		{
			name: "multiple stacks",
			layout: []string{
				"s:s1",
				"s:s2",
			},
			want: want{
				run: runExpected{
					Status: 0,
					Stdout: "/s1\n/s2\n",
				},
				events: eventsResponse{
					"s1": []string{"pending", "running", "ok"},
					"s2": []string{"pending", "running", "ok"},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			var genIdsLayout []string
			ids := []string{}
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
						ids = append(ids, id)
						layout += ":id=" + id
					}
					genIdsLayout = append(genIdsLayout, layout)
				}
			} else {
				genIdsLayout = tc.layout
			}

			s.BuildTree(genIdsLayout)
			s.Git().CommitAll("all stacks committed")
			cli := newCLI(t, filepath.Join(s.RootDir(), filepath.FromSlash(tc.workingDir)))
			uuid, err := uuid.NewRandom()
			assert.NoError(t, err)
			runid := uuid.String()
			cli.appendEnv = []string{"TM_TEST_RUN_ID=" + runid}

			runflags := []string{"--cloud-sync-deployment"}
			runflags = append(runflags, tc.runflags...)

			fixture := cli.newRunFixture(tc.runMode, s.RootDir(), runflags...)
			fixture.cmd = tc.cmd // if empty, uses the runFixture default cmd.
			result := fixture.run()
			assertRunResult(t, result, tc.want.run)
			assertRunEvents(t, runid, ids, tc.want.events)
		})
	}
}

func TestCloudSyncSkipped(t *testing.T) {
	type testcase struct {
		name      string
		endpoints map[string]bool
		want      runExpected
	}

	for _, tc := range []testcase{
		{
			name:      "all endpoints",
			endpoints: testserver.EnableAllConfig(),
		},
		{
			name: "/v1/users is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       false,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: true,
			},
			want: runExpected{
				Status:      0,
				StderrRegex: cli.DisablingCloudMessage,
			},
		},
		{
			name: "/v1/memberships is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: false,
				cloud.DeploymentsPath: true,
			},
			want: runExpected{
				Status:      0,
				StderrRegex: cli.DisablingCloudMessage,
			},
		},
		{
			name: "/v1/deployments is not working",
			endpoints: map[string]bool{
				cloud.UsersPath:       true,
				cloud.MembershipsPath: true,
				cloud.DeploymentsPath: false,
			},
			want: runExpected{
				Status:      0,
				StderrRegex: cli.DisablingCloudMessage,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fakeserver := &http.Server{
				Handler: testserver.RouterWith(tc.endpoints),
				Addr:    "localhost:3001",
			}

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
			s.BuildTree([]string{
				"s:stack:id=test",
			})
			s.Git().CommitAll("created stacks")
			tm := newCLI(t, s.RootDir())
			tm.loglevel = zerolog.WarnLevel.String()
			assertRunResult(t,
				tm.run("run", "--cloud-sync-deployment", "--", testHelperBin, "true"),
				tc.want,
			)
		})
	}
}

func assertRunEvents(t *testing.T, runid string, ids []string, events map[string][]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	expectedEvents := eventsResponse{}
	if events == nil {
		events = make(map[string][]string)
	}

	for stackpath, ev := range events {
		found := false
		for _, id := range ids {
			if strings.HasPrefix(id, strings.ToLower(strings.ReplaceAll(stackpath+"-id-", "/", "-"))) {
				expectedEvents[id] = ev
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("generated id not found for stack %s", stackpath)
		}
	}

	res, err := cloud.Request[eventsResponse](ctx, &cloud.Client{
		BaseURL:    "http://localhost:3001",
		Credential: &credential{},
	}, "GET", cloud.DeploymentsPath+"/"+testserver.DefaultOrgUUID+"/"+runid+"/events", nil)
	assert.NoError(t, err)

	if diff := cmp.Diff(res, expectedEvents); diff != "" {
		t.Logf("want: %+v", expectedEvents)
		t.Logf("got: %+v", res)
		t.Fatal(diff)
	}
}

func TestRunGithubTokenDetection(t *testing.T) {
	s := sandbox.New(t)
	git := s.Git()
	git.SetRemoteURL("origin", "https://github.com/any-org/any-repo")

	s.BuildTree([]string{
		"s:s1:id=s1",
		"s:s2:id=s2",
	})

	git.CommitAll("all files")

	fakeserver := &http.Server{
		Handler: testserver.Router(),
		Addr:    "localhost:3001",
	}

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

	t.Run("GH_TOKEN detection", func(t *testing.T) {
		tm := newCLI(t, s.RootDir())
		tm.loglevel = "debug"
		tm.appendEnv = append(tm.appendEnv, "GH_TOKEN=abcd")

		result := tm.run("run",
			"--disable-check-git-remote",
			"--cloud-sync-deployment", "--", "true")
		assertRunResult(t, result, runExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from GH_TOKEN",
		})
	})

	t.Run("GITHUB_TOKEN detection", func(t *testing.T) {
		tm := newCLI(t, s.RootDir())
		tm.appendEnv = append(tm.appendEnv, "GITHUB_TOKEN=abcd")
		tm.loglevel = "debug"

		result := tm.run("run",
			"--disable-check-git-remote",
			"--cloud-sync-deployment", "--", "true")
		assertRunResult(t, result, runExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from GITHUB_TOKEN",
		})
	})

	t.Run("GH config file detection", func(t *testing.T) {
		_, err := safeexec.LookPath("gh")
		if err != nil {
			t.Skip("gh tool not installed")
		}

		tm := newCLI(t, s.RootDir())
		tm.loglevel = "debug"
		ghConfigDir := t.TempDir()
		test.WriteFile(t, ghConfigDir, "hosts.yml", `github.com:
    user: test
    oauth_token: abcd
    git_protocol: ssh
`)
		tm.appendEnv = append(tm.appendEnv, "GH_CONFIG_DIR="+ghConfigDir)

		result := tm.run("run",
			"--disable-check-git-remote",
			"--cloud-sync-deployment", "--", "true")
		assertRunResult(t, result, runExpected{
			Status:      0,
			StderrRegex: "GitHub token obtained from oauth_token",
		})
	})
}

type credential struct{}

func (c *credential) Token() (string, error) {
	return "abcd", nil
}

type eventsResponse map[string][]string

func (res eventsResponse) Validate() error {
	return nil
}
