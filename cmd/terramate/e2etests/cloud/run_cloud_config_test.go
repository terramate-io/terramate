// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cloud/testserver"
	"github.com/terramate-io/terramate/cloud/testserver/cloudstore"
	"github.com/terramate-io/terramate/cmd/terramate/cli/clitest"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCloudConfig(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name      string
		layout    []string
		want      RunExpected
		customEnv map[string]string
	}

	const fatalErr = `FTL ` + string(clitest.ErrCloud)

	for _, tc := range []testcase{
		{
			name: "empty cloud block == no organization set",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
						}
					}
				}`,
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
			name: "not a member of selected organization",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "world"
						}
					}
				}`,
			},
			want: RunExpected{
				Status: 1,
				StderrRegexes: []string{
					`You are not a member of organization "world"`,
					fatalErr,
				},
			},
		},
		{
			name: "member of organization",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "mineiros-io"
						}
					}
				}`,
			},
			want: RunExpected{
				IgnoreStderr: true,
				Status:       0,
			},
		},
		{
			name: "cloud organization env var overrides value from config",
			layout: []string{
				"s:s1:id=s1",
				`f:cfg.tm.hcl:terramate {
					config {
						cloud {
							organization = "mineiros-io"
						}
					}
				}`,
			},
			customEnv: map[string]string{
				"TM_CLOUD_ORGANIZATION": "override",
			},
			want: RunExpected{
				Status: 1,
				StderrRegexes: []string{
					`You are not a member of organization "override"`,
					fatalErr,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store, err := cloudstore.LoadDatastore(testserverJSONFile)
			assert.NoError(t, err)

			store.UpsertOrg(cloudstore.Org{
				UUID:        "b2f153e8-ceb1-4f26-898e-eb7789869bee",
				Name:        "mineiros-io",
				DisplayName: "Mineiros",
				Status:      "active",
				Members: []cloudstore.Member{
					{
						UserUUID: store.MustGetUser("batman@terramate.io").UUID,
						Role:     "member",
						Status:   "active",
					},
				},
			})

			l, err := net.Listen("tcp", ":0")
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

			s := sandbox.New(t)
			layout := tc.layout
			if len(layout) == 0 {
				layout = []string{
					"s:stack:id=test",
				}
			}
			s.BuildTree(layout)
			s.Git().CommitAll("created stacks")
			env := RemoveEnv(os.Environ(), "CI")

			for k, v := range tc.customEnv {
				env = append(env, fmt.Sprintf("%v=%v", k, v))
			}

			env = append(env, "TMC_API_URL=http://"+l.Addr().String())
			tm := NewCLI(t, s.RootDir(), env...)

			cmd := []string{
				"run",
				"--cloud-sync-deployment",
				"--", HelperPath, "true",
			}
			AssertRunResult(t, tm.Run(cmd...), tc.want)
		})
	}
}
