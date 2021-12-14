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

package cli_test

import (
	"fmt"
	"path/filepath"
	"testing"

	hclversion "github.com/hashicorp/go-version"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/cmd/terramate/cli"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

type versionPart int

const configFile = terramate.ConfigFilename

const (
	vMajor = iota
	vMinor
	vPatch
)

var sprintf = fmt.Sprintf
var tsversion = terramate.Version()

func TestInit(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		input  []string
		force  bool
		want   runResult
	}

	for _, tc := range []testcase{
		{
			name:   "init basedir",
			layout: nil,
			force:  false,
		},
		{
			name:   "init basedir - init --force",
			layout: nil,
			force:  true,
		},
		{
			name:   "same version stack",
			layout: []string{"s:same-version"},
			input:  []string{"same-version"},
			force:  false,
		},
		{
			name:   "same version stack - init --force",
			layout: []string{"s:same-version"},
			input:  []string{"same-version"},
			force:  true,
		},
		{
			name: "multiple same version stacks",
			layout: []string{
				"s:same-version-1",
				"s:same-version-2",
				"s:same-version-3",
			},
			input: []string{"same-version-1", "same-version-2", "same-version-3"},
		},
		{
			name: "not compatible stack",
			layout: []string{
				"s:other-version:version=~> 9999.9999.9999",
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				IgnoreStderr: true,
				Error:        cli.ErrInit,
			},
		},
		{
			name: "not compatible stack - init --forced",
			layout: []string{
				"s:other-version:version=~> 9999.9999.9999",
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "multiple stacks, one incompatible version stack - fails",
			layout: []string{
				"s:other-version:version=~> 9999.9999.9999",
				"s:stack1",
				"s:stack2",
			},
			input: []string{"stack1", "stack2", "other-version"},
			force: false,
			want: runResult{
				IgnoreStderr: true,
				Error:        cli.ErrInit,
			},
		},
		{
			name: "bigger version patch - fails",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vPatch)),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger version patch - init --forced",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vPatch)),
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "bigger version minor - fails",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vMinor)),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger version minor - init --forced",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vMinor)),
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "bigger version major - fails",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vMajor)),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger version major - init --forced",
			layout: []string{
				sprintf("s:other-version:version=~> %s", incVersion(t, tsversion, vMajor)),
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "lower than terramate version - fails",
			layout: []string{
				"s:other-version:version=< 0.0.1",
			},
			input: []string{"other-version"},
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger than default constraint version - fails",
			layout: []string{
				"s:other-version:version=> 999.0.0",
			},
			input: []string{"other-version"},
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, s.BaseDir())
			args := []string{"init"}
			if tc.force {
				args = append(args, "--force")
			}
			if len(tc.input) > 0 {
				args = append(args, tc.input...)
			}
			assertRunResult(t, cli.run(args...), tc.want)

			if tc.want.Error != nil {
				return
			}

			for _, path := range tc.input {
				data := test.ReadFile(t, s.BaseDir(), filepath.Join(path, configFile))
				got, err := hcl.Parse("TestInitHCL", data)
				assert.NoError(t, err, "parsing terramate file")

				want := hcl.Terramate{
					RequiredVersion: terramate.DefaultVersionConstraint(),
				}
				test.AssertTerramateBlock(t, *got, want)
			}
		})
	}
}

func TestInitNonExistingDir(t *testing.T) {
	s := sandbox.New(t)
	c := newCLI(t, s.BaseDir())
	assertRunResult(t, c.run("init", test.NonExistingDir(t)), runResult{
		Error:        cli.ErrInit,
		IgnoreStderr: true,
	})
}

func incVersion(t *testing.T, v string, pos versionPart) string {
	semver, err := hclversion.NewSemver(v)
	assert.NoError(t, err)
	segs := semver.Segments()
	if len(segs) == 1 {
		segs = append(segs, 0)
	}
	if len(segs) == 2 {
		segs = append(segs, 0)
	}
	segs[pos]++

	return fmt.Sprintf("%d.%d.%d", segs[0], segs[1], segs[2])
}
