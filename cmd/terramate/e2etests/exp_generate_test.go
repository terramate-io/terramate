// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestGenerateDebug(t *testing.T) {
	type (
		file struct {
			path string
			body fmt.Stringer
		}
		testcase struct {
			name    string
			layout  []string
			wd      string
			configs []file
			want    runExpected
		}
	)
	t.Parallel()

	testcases := []testcase{
		{
			name: "empty project",
			want: runExpected{},
		},
		{
			name: "stacks with no codegen",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			want: runExpected{},
		},
		{
			name: "stacks with codegen with root as working dir",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", true),
							Str("content", "data"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Bool("condition", true),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
				{
					path: "stack-2/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Bool("condition", true),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
			},
			want: runExpected{
				Stdout: `/stack-1/file.hcl origin: /stack-1/config.tm:1,1-6,2
/stack-1/file.txt origin: /config.tm:5,1-8,2
/stack-2/file.hcl origin: /stack-2/config.tm:1,1-6,2
/stack-2/file.txt origin: /config.tm:5,1-8,2
`,
			},
		},
		{
			name: "stacks with codegen with stack as working dir",
			layout: []string{
				"s:stack-1",
				"s:stack-1/dir/child",
				"s:stack-2",
			},
			wd: "stack-1",
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("file.txt"),
							Str("content", "data"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
			},
			want: runExpected{
				Stdout: `/stack-1/file.hcl origin: /stack-1/config.tm:1,1-5,2
/stack-1/file.txt origin: /config.tm:5,1-7,2
/stack-1/dir/child/file.hcl origin: /stack-1/config.tm:1,1-5,2
/stack-1/dir/child/file.txt origin: /config.tm:5,1-7,2
`,
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			root := s.RootEntry()

			for _, config := range tc.configs {
				root.CreateFile(config.path, config.body.String())
			}

			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			assertRunResult(t, ts.run("experimental", "generate", "debug"), tc.want)
		})
	}
}

func TestGenerateDebugWithChanged(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stack-1",
		"s:stack-2",
		"d:no-stack",
	})
	root := s.RootEntry()

	root.CreateFile("config.tm", Doc(
		GenerateFile(
			Labels("file.txt"),
			Str("content", "data"),
		),
		GenerateHCL(
			Labels("file.hcl"),
			Content(
				Str("content", "data"),
			),
		),
	).String())

	g := s.Git()
	g.CommitAll("root configs")
	g.Push("main")

	g.CheckoutNew("change-stack-1")

	stack1 := s.DirEntry("stack-1")
	stack1.CreateFile("change.txt", "changed stack")

	g.CommitAll("changed stack-1")

	want := `/stack-1/file.hcl origin: /config.tm:4,1-8,2
/stack-1/file.txt origin: /config.tm:1,1-3,2
`
	ts := newCLI(t, s.RootDir())
	assertRunResult(t, ts.run("experimental", "generate", "debug", "--changed"), runExpected{
		Stdout: want,
	})

	ts = newCLI(t, filepath.Join(s.RootDir(), "stack-1"))
	assertRunResult(t, ts.run("experimental", "generate", "debug", "--changed"), runExpected{
		Stdout: want,
	})

	ts = newCLI(t, filepath.Join(s.RootDir(), "stack-2"))
	assertRunResult(t, ts.run("experimental", "generate", "debug", "--changed"), runExpected{})

	ts = newCLI(t, filepath.Join(s.RootDir(), "no-stack"))
	assertRunResult(t, ts.run("experimental", "generate", "debug", "--changed"), runExpected{})
}
