// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestEnsureStackID(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		wd     string
	}

	for _, tc := range []testcase{
		{
			name: "single stack at root with id",
			layout: []string{
				`s:.:id=test`,
			},
		},
		{
			name: "single stack at root without id",
			layout: []string{
				`s:.`,
			},
		},
		{
			name: "single stack at root without id but wd not at root",
			layout: []string{
				`d:some/deep/dir/for/test`,
				`s:.`,
			},
			wd: `/some/deep/dir/for/test`,
		},
		{
			name: "mix of multiple stacks with and without id",
			layout: []string{
				`s:s1`,
				`s:s1/a1:id=test`,
				`s:s2`,
				`s:s3/a3:id=test2`,
				`s:s3/a1`,
				`s:a/b/c/d/e/f/g/h/stack`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testEnsureStackID(t, tc.wd, tc.layout)
		})
	}
}

func testEnsureStackID(t *testing.T, wd string, layout []string) {
	s := sandbox.New(t)
	s.BuildTree(layout)
	if wd == "" {
		wd = s.RootDir()
	} else {
		wd = filepath.Join(s.RootDir(), filepath.FromSlash(wd))
	}
	tm := newCLI(t, wd)
	assertRunResult(
		t,
		tm.run("experimental", "ensure-stack-id"),
		runExpected{
			Status:       0,
			IgnoreStdout: true,
		},
	)

	s.ReloadConfig()
	for _, stackElem := range s.LoadStacks() {
		if stackElem.ID == "" {
			t.Fatalf("stack.id not generated for stack %s", stackElem.Dir())
		}
	}
}
