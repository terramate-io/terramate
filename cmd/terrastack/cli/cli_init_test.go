package cli_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
	"github.com/mineiros-io/terrastack/test"
	"github.com/mineiros-io/terrastack/test/sandbox"
)

const otherVersionContent = `
terrastack {
	required_version = "~> 9999.9999.9999"
}
`

const configFile = terrastack.ConfigFilename

var sprintf = fmt.Sprintf

func TestInit(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		paths  []string
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
			name:   "same version stack",
			layout: []string{"s:same-version"},
			paths:  []string{"same-version"},
			force:  false,
		},
		{
			name:   "same version stack - init --force",
			layout: []string{"s:same-version"},
			paths:  []string{"same-version"},
			force:  true,
		},
		{
			name: "multiple same version stacks",
			layout: []string{
				"s:same-version-1",
				"s:same-version-2",
				"s:same-version-3",
			},
			paths: []string{"same-version-1", "same-version-2", "same-version-3"},
		},
		{
			name: "other version stack - not forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, otherVersionContent),
			},
			paths: []string{"other-version"},
			force: false,
			want: runResult{
				IgnoreStderr: true,
				Error:        cli.ErrInit,
			},
		},
		{
			name: "other version stack - forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, otherVersionContent),
			},
			paths: []string{"other-version"},
			force: true,
		},
		{
			name: "multiple stacks, one incompatible version stack - not forced - fails",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, otherVersionContent),
				"s:stack1",
				"s:stack2",
			},
			paths: []string{"stack1", "stack2", "other-version"},
			force: false,
			want: runResult{
				IgnoreStderr: true,
				Error:        cli.ErrInit,
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
			if len(tc.paths) > 0 {
				args = append(args, tc.paths...)
			}
			assertRunResult(t, cli.run(args...), tc.want)

			if tc.want.Error != nil {
				return
			}

			for _, path := range tc.paths {
				data := test.ReadFile(t, s.BaseDir(), filepath.Join(path, configFile))
				p := hhcl.NewParser()
				got, err := p.Parse("TestInitHCL", data)
				assert.NoError(t, err, "parsing terrastack file")

				want := hcl.Terrastack{
					RequiredVersion: terrastack.Version(),
				}
				if *got != want {
					t.Fatalf("terrastack file differs: want[%+v] != got[%+v]", want, *got)
				}
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
