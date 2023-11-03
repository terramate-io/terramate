// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cliconfig_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/test/sandbox"
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
			cfg:  `disable_checkpoint = 1`,
			want: want{
				err: errors.E(cliconfig.ErrInvalidAttributeType),
			},
		},
		{
			name: "disable_checkpoint_signature with wrong type",
			cfg:  `disable_checkpoint_signature = 1`,
			want: want{
				err: errors.E(cliconfig.ErrInvalidAttributeType),
			},
		},
		{
			name: "unrecognized attribute",
			cfg:  `unrecognized = true`,
			want: want{
				err: errors.E(cliconfig.ErrUnrecognizedAttribute),
			},
		},
		{
			name: "disable_checkpoint = anytrue(true) - TM functions not supported",
			cfg:  `disable_checkpoint = tm_anytrue(false, true)`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "disable_checkpoint = anytrue(true) - TF functions not supported",
			cfg:  `disable_checkpoint = anytrue(false, true)`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "valid disable_checkpoint",
			cfg:  `disable_checkpoint = true`,
			want: want{
				cfg: cliconfig.Config{
					DisableCheckpoint: true,
				},
			},
		},
		{
			name: "valid disable_checkpoint_signature",
			cfg:  `disable_checkpoint_signature = true`,
			want: want{
				cfg: cliconfig.Config{
					DisableCheckpointSignature: true,
				},
			},
		},
		{
			name: "disable_checkpoint and disable_checkpoint_signature",
			cfg: `disable_checkpoint = true
			disable_checkpoint_signature = true`,
			want: want{
				cfg: cliconfig.Config{
					DisableCheckpointSignature: true,
					DisableCheckpoint:          true,
				},
			},
		},
		{
			name: "disable_checkpoint and disable_checkpoint_signature -- diff values",
			cfg: `disable_checkpoint = true
			disable_checkpoint_signature = false`,
			want: want{
				cfg: cliconfig.Config{
					DisableCheckpointSignature: false,
					DisableCheckpoint:          true,
				},
			},
		},
		{
			name: "set user_terramate_dir to an invalid value",
			cfg:  `user_terramate_dir = true`,
			want: want{
				err: errors.E(cliconfig.ErrInvalidAttributeType),
			},
		},
		{
			name: "set user_terramate_dir from funcall -- not supported",
			cfg:  `user_terramate_dir = tm_upper("/")`,
			want: want{
				err: errors.E(eval.ErrEval),
			},
		},
		{
			name: "set user_terramate_dir -- any string works",
			cfg:  `user_terramate_dir = "/tmp"`,
			want: want{
				cfg: cliconfig.Config{
					UserTerramateDir: "/tmp",
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, false)
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
