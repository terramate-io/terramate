// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptInfo(t *testing.T) {
	t.Parallel()

	layout := []string{"d:stacks"}

	addStackWithScripts := func(path string, scripts []string) {
		layout = append(layout, fmt.Sprintf("s:%s", path))

		var content string

		for _, name := range scripts {
			content += fmt.Sprintf(`script "%s" {
				description = "%s at /%s"
				job {
					commands = [
						["echo", "1a"],
						["echo", "1b"]
					]
				}
				job {
					command = ["echo", "2"]
				}
			}
			`, name, name, path)
		}

		layout = append(layout, fmt.Sprintf(`f:%s/script.tm:%s`, path, content))
	}

	mkExpected := func(name, path, loc string, stacks []string) string {
		stackstr := ""
		for _, s := range stacks {
			stackstr += fmt.Sprintf("\n  %s", s)
		}
		if stackstr == "" {
			stackstr = " (none)"
		}

		defstr := fmt.Sprintf("%s/script.tm:%s", path, loc)
		if path != "" {
			defstr = "/" + defstr
		}

		return fmt.Sprintf(`Definition: %s
Description: "%s at /%s"
Stacks:%s
Jobs:
  * ["echo","1a"]
    ["echo","1b"]
  * ["echo","2"]

`, defstr, name, path, stackstr)
	}

	addStackWithScripts("", []string{"root"})
	addStackWithScripts("stacks/a", []string{"deploy", "other"})
	addStackWithScripts("stacks/a/a1", []string{"other2"})
	addStackWithScripts("stacks/a/a1/a2", []string{"deploy"})
	addStackWithScripts("stacks/b", []string{"deploy", "other"})
	addStackWithScripts("stacks/bb", []string{"other3"})

	s := sandbox.New(t)
	s.BuildTree(layout)

	s.RootEntry().CreateConfig(`
		terramate {
			config {
				experiments = ["scripts"]
			}
	  	}
	`)

	git := s.Git()
	git.CommitAll("everything")

	type testcase struct {
		script string
		dir    string
		want   RunExpected
	}

	for _, tc := range []testcase{
		{
			script: "not_found",
			dir:    "",
			want: RunExpected{
				StderrRegex: "script not found",
				Status:      1,
			},
		},
		{
			script: "root",
			dir:    "",
			want: RunExpected{
				Stdout: mkExpected("root", "", "1,1-12,5", []string{
					"/",
					"/stacks/a",
					"/stacks/a/a1",
					"/stacks/a/a1/a2",
					"/stacks/b",
					"/stacks/bb",
				}),
			},
		},
		{
			script: "deploy",
			dir:    "",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/a", "1,1-12,5", []string{
					"/stacks/a",
					"/stacks/a/a1",
				}) + mkExpected("deploy", "stacks/a/a1/a2", "1,1-12,5", []string{
					"/stacks/a/a1/a2",
				}) + mkExpected("deploy", "stacks/b", "1,1-12,5", []string{
					"/stacks/b",
				}),
			},
		},
		{
			script: "deploy",
			dir:    "stacks/a/a1",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/a", "1,1-12,5", []string{
					"/stacks/a/a1",
				}) + mkExpected("deploy", "stacks/a/a1/a2", "1,1-12,5", []string{
					"/stacks/a/a1/a2",
				}) + mkExpected("deploy", "stacks/b", "1,1-12,5", []string{}),
			},
		},
		{
			script: "deploy",
			dir:    "stacks/a/a1/a2",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/a", "1,1-12,5", []string{}) +
					mkExpected("deploy", "stacks/a/a1/a2", "1,1-12,5", []string{
						"/stacks/a/a1/a2",
					}) + mkExpected("deploy", "stacks/b", "1,1-12,5", []string{}),
			},
		},
		{
			script: "deploy",
			dir:    "stacks/b",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/a", "1,1-12,5", []string{}) +
					mkExpected("deploy", "stacks/a/a1/a2", "1,1-12,5", []string{}) +
					mkExpected("deploy", "stacks/b", "1,1-12,5", []string{
						"/stacks/b",
					}),
			},
		},
		{
			script: "other2",
			dir:    "stacks/b",
			want: RunExpected{
				Stdout: mkExpected("other2", "stacks/a/a1", "1,1-12,5", []string{}),
			},
		},
		{
			script: "other3",
			dir:    "stacks/bb",
			want: RunExpected{
				Stdout: mkExpected("other3", "stacks/bb", "1,1-12,5", []string{
					"/stacks/bb",
				}),
			},
		},
		{
			script: "other3",
			dir:    "stacks/b",
			want: RunExpected{
				Stdout: mkExpected("other3", "stacks/bb", "1,1-12,5", []string{}),
			},
		},
	} {
		tc := tc
		name := fmt.Sprintf("%v in %v", tc.script, tc.dir)
		name = strings.ReplaceAll(name, "/", "_")
		t.Run(name, func(t *testing.T) {
			wd := s.RootDir()
			if tc.dir != "" {
				wd = filepath.Join(wd, tc.dir)
			}

			cli := NewCLI(t, wd)
			AssertRunResult(t, cli.Run("script", "info", "--", tc.script), tc.want)
		})

	}
}
