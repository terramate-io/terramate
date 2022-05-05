// Copyright 2021 Mineiros GmbH
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

package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/cmd/terramate/cli"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStacksInit(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		input  []string
		want   runExpected
	}

	for _, tc := range []testcase{
		{
			name:   "init basedir",
			layout: nil,
		},
		{
			name: "initialize subdirs",
			layout: []string{
				"d:stack",
				"d:stacks/stack-1",
			},
			input: []string{"stack", "stacks/stack-1"},
		},
		{
			name:   "initialized stacks are ignored",
			layout: []string{"s:stack"},
			input:  []string{"stack"},
		},
		{
			name:   "dot directories are not allowed",
			layout: []string{"d:.stack"},
			input:  []string{".stack"},
			want: runExpected{
				StderrRegex: "dot directories are not allowed",
				Status:      1,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.RootDir())
			args := []string{}
			if len(tc.input) > 0 {
				args = append(args, tc.input...)
			}
			assertRunResult(t, cli.initStack(args...), tc.want)

			if tc.want.Status != 0 {
				return
			}

			for _, path := range tc.input {
				dir := filepath.Join(s.RootDir(), path)
				got, err := hcl.ParseDir(dir)
				assert.NoError(t, err, "parsing terramate file")

				want := hcl.Config{
					Stack: &hcl.Stack{},
				}
				test.AssertTerramateConfig(t, got, want)
			}
		})
	}
}

func TestInitNonExistingDir(t *testing.T) {
	s := sandbox.New(t)
	c := newCLI(t, s.RootDir())
	assertRunResult(t, c.initStack(test.NonExistingDir(t)), runExpected{
		StderrRegex: string(cli.ErrInit),
		Status:      1,
	})
}

func TestInitFailInitializeChildOfStack(t *testing.T) {
	s := sandbox.New(t)
	c := newCLI(t, s.RootDir())
	parent := test.Mkdir(t, s.RootDir(), "parent-stack")
	child := test.Mkdir(t, parent, "child-stack")
	assertRun(t, c.initStack(parent))
	assertRunResult(t, c.initStack(child), runExpected{
		StderrRegex: string(cli.ErrInit),
		Status:      1,
	})
}

func TestInitFailInitializeParentOfChildStack(t *testing.T) {
	s := sandbox.New(t)
	c := newCLI(t, s.RootDir())
	parent := test.Mkdir(t, s.RootDir(), "parent-stack")
	child := test.Mkdir(t, parent, "child-stack")
	assertRun(t, c.initStack(child))
	assertRunResult(t, c.initStack(parent), runExpected{
		StderrRegex: string(cli.ErrInit),
		Status:      1,
	})
}
