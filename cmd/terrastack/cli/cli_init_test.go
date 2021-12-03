package cli_test

import (
	"fmt"
	"path/filepath"
	"testing"

	hclversion "github.com/hashicorp/go-version"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terrastack"
	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/mineiros-io/terrastack/hcl/hhcl"
	"github.com/mineiros-io/terrastack/test"
	"github.com/mineiros-io/terrastack/test/sandbox"
)

type versionPart int

const configFile = terrastack.ConfigFilename

const (
	vMajor = iota
	vMinor
	vPatch
)

var sprintf = fmt.Sprintf
var tsversion = terrastack.Version()

func TestInit(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		input  []string
		force  bool
		want   runResult
	}

	const bigVersionContent = `
terrastack {
	required_version = "~> 9999.9999.9999"
}
`

	var biggerPatchVersionContent = sprintf(`
terrastack {
	required_version = "~> %s"
}
`, incVersion(t, tsversion, vPatch))

	var biggerMinorVersionContent = sprintf(`
terrastack {
	required_version = "~> %s"
}
`, incVersion(t, tsversion, vMinor))

	for _, tc := range []testcase{
		{
			name:   "init basedir",
			layout: nil,
			force:  false,
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
			name: "other version stack - not forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, bigVersionContent),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				IgnoreStderr: true,
				Error:        cli.ErrInit,
			},
		},
		{
			name: "other version stack - forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, bigVersionContent),
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "multiple stacks, one incompatible version stack - not forced - fails",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, bigVersionContent),
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
				sprintf("f:other-version/%s:%s", configFile, biggerPatchVersionContent),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger version patch - forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, biggerPatchVersionContent),
			},
			input: []string{"other-version"},
			force: true,
		},
		{
			name: "bigger version minor - fails",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, biggerMinorVersionContent),
			},
			input: []string{"other-version"},
			force: false,
			want: runResult{
				Error:        cli.ErrInit,
				IgnoreStderr: true,
			},
		},
		{
			name: "bigger version minor - forced",
			layout: []string{
				sprintf("f:other-version/%s:%s", configFile, biggerPatchVersionContent),
			},
			input: []string{"other-version"},
			force: true,
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
				p := hhcl.NewParser()
				got, err := p.Parse("TestInitHCL", data)
				assert.NoError(t, err, "parsing terrastack file")

				want := hcl.Terrastack{
					RequiredVersion: fmt.Sprintf("%s %s",
						terrastack.DefaultInitConstraint,
						terrastack.Version()),
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
