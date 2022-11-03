// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modvendor_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
	"go.lsp.dev/uri"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestVendorEvents(t *testing.T) {
	type file struct {
		path string
		body fmt.Stringer
	}
	type repository struct {
		name  string
		files []file
	}
	// progressEvent represents an modvendor.ProgressEvent
	// but without a parsed module since we need to fix the module
	// URI before parsing it during test execution (can't be done
	// ahead of time).
	type progressEvent struct {
		message   string
		targetDir string
		module    string
	}
	type testcase struct {
		name         string
		source       string
		vendorDir    string
		repositories []repository
		want         []progressEvent
	}

	t.Parallel()

	const progressMessage = "downloading"

	tcases := []testcase{
		{
			name:      "unknown source produce event",
			source:    "git::{{.}}/unknown?ref=branch",
			vendorDir: "/vendor",
			want: []progressEvent{
				{
					message:   progressMessage,
					targetDir: "/vendor/{{.}}/unknown/branch",
					module:    "git::{{.}}/unknown?ref=branch",
				},
			},
		},
		{
			name: "source with no deps",
			repositories: []repository{
				{
					name: "test",
				},
			},
			source:    "git::{{.}}/test?ref=main",
			vendorDir: "/modules",
			want: []progressEvent{
				{
					message:   progressMessage,
					targetDir: "/modules/{{.}}/test/main",
					module:    "git::{{.}}/test?ref=main",
				},
			},
		},
		{
			name: "source with transitive deps",
			repositories: []repository{
				{
					name: "test",
				},
				{
					name: "test2",
					files: []file{
						{
							path: "config.tf",
							body: Module(
								Labels("test"),
								Str("source", "git::{{.}}/test?ref=main"),
							),
						},
					},
				},
				{
					name: "test3",
					files: []file{
						{
							path: "config.tf",
							body: Module(
								Labels("test2"),
								Str("source", "git::{{.}}/test2?ref=main"),
							),
						},
					},
				},
			},
			source:    "git::{{.}}/test3?ref=main",
			vendorDir: "/any",
			want: []progressEvent{
				{
					message:   progressMessage,
					targetDir: "/any/{{.}}/test3/main",
					module:    "git::{{.}}/test3?ref=main",
				},
				{
					message:   progressMessage,
					targetDir: "/any/{{.}}/test2/main",
					module:    "git::{{.}}/test2?ref=main",
				},
				{
					message:   progressMessage,
					targetDir: "/any/{{.}}/test/main",
					module:    "git::{{.}}/test?ref=main",
				},
			},
		},
		{
			name: "transitive deps with unknown repos",
			repositories: []repository{
				{
					name: "test",
				},
				{
					name: "test2",
					files: []file{
						{
							path: "config.tf",
							body: Doc(
								Module(
									Labels("unknown"),
									Str("source", "git::{{.}}/unknown?ref=unknown"),
								),
								Module(
									Labels("test"),
									Str("source", "git::{{.}}/test?ref=main"),
								),
							),
						},
					},
				},
				{
					name: "test3",
					files: []file{
						{
							path: "config.tf",
							body: Module(
								Labels("test2"),
								Str("source", "git::{{.}}/test2?ref=main"),
							),
						},
					},
				},
			},
			source:    "git::{{.}}/test3?ref=main",
			vendorDir: "/modules",
			want: []progressEvent{
				{
					message:   progressMessage,
					targetDir: "/modules/{{.}}/test3/main",
					module:    "git::{{.}}/test3?ref=main",
				},
				{
					message:   progressMessage,
					targetDir: "/modules/{{.}}/test2/main",
					module:    "git::{{.}}/test2?ref=main",
				},
				{
					message:   progressMessage,
					targetDir: "/modules/{{.}}/unknown/unknown",
					module:    "git::{{.}}/unknown?ref=unknown",
				},
				{
					message:   progressMessage,
					targetDir: "/modules/{{.}}/test/main",
					module:    "git::{{.}}/test?ref=main",
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			repositoresSbx := sandbox.NoGit(t)
			reposURI := uri.File(repositoresSbx.RootDir())

			for _, repo := range tcase.repositories {
				repositoriesRoot := repositoresSbx.RootDir()
				repoRoot := filepath.Join(repositoriesRoot, repo.name)

				test.MkdirAll(t, repoRoot)
				git := sandbox.NewGit(t, repoRoot)
				git.Init()

				for _, file := range repo.files {
					// if the body has template {{.}} we need to replace
					// with proper references to the repository root
					// we just created.
					body := file.body.String()
					body = applyConfigTemplate(t, body, reposURI)
					test.WriteFile(t, repositoriesRoot,
						filepath.Join(repo.name, file.path),
						body,
					)
				}

				if len(repo.files) > 0 {
					git.CommitAll("files updated")
				}
			}

			wantEvents := make([]modvendor.ProgressEvent, len(tcase.want))
			for i, w := range tcase.want {
				// We need to fix the wanted events with the proper
				// git URL/path, but that is now know before execution
				// since repositories are created dynamically.
				module := applyConfigTemplate(t, w.module, reposURI)
				targetDir := applyConfigTemplate(t, w.targetDir, reposURI.Filename())
				targetDir = filepath.ToSlash(targetDir)

				wantEvents[i] = modvendor.ProgressEvent{
					Message:   w.message,
					TargetDir: project.NewPath(targetDir),
					Module:    test.ParseSource(t, module),
				}
			}

			source := applyConfigTemplate(t, tcase.source, reposURI)
			modsrc := test.ParseSource(t, source)

			s := sandbox.New(t)
			vendorDir := project.NewPath(tcase.vendorDir)

			eventsHandled := make(chan struct{})
			eventsStream := modvendor.NewEventStream()
			gotEvents := []modvendor.ProgressEvent{}

			go func() {
				for event := range eventsStream {
					gotEvents = append(gotEvents, event)
				}
				close(eventsHandled)
			}()

			modvendor.Vendor(s.RootDir(), vendorDir, modsrc, eventsStream)
			close(eventsStream)
			<-eventsHandled

			test.AssertDiff(t, gotEvents, wantEvents)
		})
	}
}
