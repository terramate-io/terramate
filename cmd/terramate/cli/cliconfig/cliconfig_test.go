package cliconfig_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoad(t *testing.T) {
	type want struct {
		err error
		cfg cliconfig.Config
	}

	type testcase struct {
		name string
		cfg  string
		want want
	}

	for _, tc := range []testcase{
		{
			name: "empty config",
			cfg:  ``,
		},
		{
			name: "empty config",
			cfg:  `wrong`,
			want: want{
				err: errors.E(hcl.ErrHCLSyntax),
			},
		},
		{
			name: "disable_checkpoint with wrong type",
			cfg: `disable_checkpoint = 1
			`,
			want: want{
				err: errors.E(cliconfig.ErrInvalidAttributeType),
			},
		},
		{
			name: "disable_checkpoint_signature with wrong type",
			cfg: `disable_checkpoint_signature = 1
			`,
			want: want{
				err: errors.E(cliconfig.ErrInvalidAttributeType),
			},
		},
		{
			name: "unrecognized attribute",
			cfg: `unrecognized = true
			`,
			want: want{
				err: errors.E(cliconfig.ErrUnknownAttribute),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree([]string{
				"d:home",
			})
			homeEntry := s.DirEntry("home")
			file := homeEntry.CreateFile(cliconfig.Filename, tc.cfg)

			cfg, err := cliconfig.LoadFrom(file.HostPath())
			errtest.Assert(t, err, tc.want.err)
			if err != nil {
				return
			}
			assertion := assert.New(t, assert.Fatal, "comparing CLI config")
			assertion.Partial(cfg, tc.want.cfg)
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
