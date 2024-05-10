// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptList(t *testing.T) {
	t.Parallel()

	layout := []string{"d:stacks"}

	addStackWithScripts := func(path string, scripts []string) {
		layout = append(layout,
			fmt.Sprintf("s:%s", path))

		var content string

		for _, name := range scripts {
			content += fmt.Sprintf(`script "%s" {
				description = "%s at /%s"
				job {
					command = ["echo", "hello"]
				}
			}
			`, name, name, path)
		}

		layout = append(layout,
			fmt.Sprintf(`f:%s/script.tm:%s`, path, content))
	}

	mkExpected := func(name, path string, count int) string {
		if count != 0 {
			return fmt.Sprintf(`%s
  Description: "%s at /%s"
  Defined at /%s
    (+%d more)

`, name, name, path, path, count)

		}

		return fmt.Sprintf(`%s
  Description: "%s at /%s"
  Defined at /%s

`, name, name, path, path)
	}

	addStackWithScripts("stacks/a", []string{"deploy", "obliterate"})
	addStackWithScripts("stacks/b", []string{"deploy", "crush"})
	addStackWithScripts("stacks/b/b1", []string{"deploy", "crush"})
	addStackWithScripts("stacks/b/b1/b2", []string{"deploy", "break"})
	addStackWithScripts("stacks/c", []string{"deploy"})

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
		dir  string
		want RunExpected
	}

	for _, tc := range []testcase{
		{
			dir: "",
			want: RunExpected{
				Stdout: mkExpected("break", "stacks/b/b1/b2", 0) +
					mkExpected("crush", "stacks/b", 1) +
					mkExpected("deploy", "stacks/a", 4) +
					mkExpected("obliterate", "stacks/a", 0),
			},
		},
		{
			dir: "stacks",
			want: RunExpected{
				Stdout: mkExpected("break", "stacks/b/b1/b2", 0) +
					mkExpected("crush", "stacks/b", 1) +
					mkExpected("deploy", "stacks/a", 4) +
					mkExpected("obliterate", "stacks/a", 0),
			},
		},
		{
			dir: "stacks/a",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/a", 0) +
					mkExpected("obliterate", "stacks/a", 0),
			},
		},
		{
			dir: "stacks/b",
			want: RunExpected{
				Stdout: mkExpected("break", "stacks/b/b1/b2", 0) +
					mkExpected("crush", "stacks/b", 1) +
					mkExpected("deploy", "stacks/b", 2),
			},
		},
		{
			dir: "stacks/b/b1",
			want: RunExpected{
				Stdout: mkExpected("break", "stacks/b/b1/b2", 0) +
					mkExpected("crush", "stacks/b", 1) +
					mkExpected("deploy", "stacks/b", 2),
			},
		},
		{
			dir: "stacks/b/b1/b2",
			want: RunExpected{
				Stdout: mkExpected("break", "stacks/b/b1/b2", 0) +
					mkExpected("crush", "stacks/b", 1) +
					mkExpected("deploy", "stacks/b", 2),
			},
		},
		{
			dir: "stacks/c",
			want: RunExpected{
				Stdout: mkExpected("deploy", "stacks/c", 0),
			},
		},
	} {
		tc := tc
		wd := s.RootDir()
		if tc.dir != "" {
			wd = filepath.Join(wd, tc.dir)
		}

		cli := NewCLI(t, wd)
		AssertRunResult(t, cli.Run("script", "list"), tc.want)
	}
}
