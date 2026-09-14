// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package versions_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/versions"
)

func TestTerramateVersionConstraints(t *testing.T) {
	t.Parallel()

	type testcase struct {
		version     string
		constraint  string
		prereleases bool
		want        error
	}

	for _, tc := range []testcase{
		{
			version:    "0.0.0",
			constraint: "0.0.0",
		},
		{
			version:     "0.0.0",
			constraint:  "0.0.0",
			prereleases: true,
			// apparentlymart/go-version does not match against the Underspecified version.
			want: errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3",
			constraint: "~> 1.2.3",
		},
		{
			version:     "1.2.3",
			constraint:  "~> 1.2.3",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "~> 1.2",
		},
		{
			version:     "1.2.3",
			constraint:  "~> 1.2",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "~> 1",
		},
		{
			version:     "1.2.3",
			constraint:  "~> 1",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "> 1",
		},
		{
			version:     "1.2.3",
			constraint:  "> 1",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2",
		},
		{
			version:     "1.2.3",
			constraint:  "> 1.2",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2.2",
		},
		{
			version:     "1.2.3",
			constraint:  "> 1.2.2",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2.3",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3",
			constraint:  "> 1.2.3",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3",
			constraint: "= 1.2.3",
		},
		{
			version:     "1.2.3",
			constraint:  "= 1.2.3",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "< 2",
		},
		{
			version:     "1.2.3",
			constraint:  "< 2",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "< 1.3",
		},
		{
			version:     "1.2.3",
			constraint:  "< 1.3",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "< 1.2.4",
		},
		{
			version:     "1.2.3",
			constraint:  "< 1.2.4",
			prereleases: true,
		},
		{
			version:    "1.2.3",
			constraint: "< 1.2.3",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3",
			constraint:  "< 1.2.3",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3",
			constraint: "<= 1.2.3",
		},
		{
			version:     "1.2.3",
			constraint:  "<= 1.2.3",
			prereleases: true,
		},
		// if prerelease=false, then prereleases are not selected if not present
		// in the constraint
		{
			version:    "1.2.3-dev",
			constraint: "~> 1",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  "~> 1",
			prereleases: true,
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  ">= 1",
			prereleases: true,
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1.2",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  ">= 1.2",
			prereleases: true,
		},
		{
			version:    "1.2.3-alpha",
			constraint: "> 1.2.2, < 1.2.3",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-alpha",
			constraint:  "> 1.2.2, < 1.2.3",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-alpha",
			constraint:  "~> 1.2.2",
			prereleases: true,
			//want:       errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1.2.3",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  ">= 1.2.3",
			prereleases: true,
		},
		{
			version:     "1.2.3-dev",
			constraint:  "< 1.2.3",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3-dev",
			constraint: "~> 1.2",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  "~> 1.2",
			prereleases: true,
		},
		{
			version:    "1.2.3-dev",
			constraint: "~> 1.2.3",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-dev",
			constraint:  "~> 1.2.2",
			prereleases: true,
		},
		{
			version:    "1.2.3-aaa",
			constraint: "~> 1.2.3-aaa", // matches exactly
		},
		{
			version:     "1.2.3-aaa",
			constraint:  "~> 1.2.3-aaa", // matches exactly
			prereleases: true,
		},
		{
			version:    "1.2.3-aaa",
			constraint: "> 1.2.3-aab",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "1.2.3-aaa",
			constraint:  "> 1.2.3-aab",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3-aab",
			constraint: "~> 1.2.3-aaa",
		},
		{
			version:     "1.2.3-aab",
			constraint:  "~> 1.2.3-aaa",
			prereleases: true,
		},
		{
			version:    "1.2.3-alpha",
			constraint: "< 1.2.3-beta",
		},
		{
			version:    "1.0.1-beta",
			constraint: ">= 1.0.1-alpha",
		},
		{
			version:     "1.2.3-alpha",
			constraint:  "< 1.2.3-beta", // match because beta > alpha
			prereleases: true,
		},
		{
			version:    "1.2.3-zzz",
			constraint: "> 1, > 1.2.3-aaa", // doesnt match because > 1 never matches pre-releases
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:    "1.2.3-zzz",
			constraint: "> 1.2.3-aaa", // match because zzz > aaa
		},
		{
			// by default matches pre-release and ignores metadata
			version:    "1.2.3-dev",
			constraint: "~> 1.2.3-dev+metadata",
		},
		{
			// metadata is ignored
			version:     "1.2.3-dev",
			constraint:  "~> 1.2.3-dev+metadata",
			prereleases: true,
		},
		{
			version:    "1.2.3-zzz+aaa",
			constraint: ">= 1.2.3-zzz",
		},
		{
			version:     "1.2.3-zzz+aaa",
			constraint:  ">= 1.2.3-zzz+zzz",
			prereleases: true,
		},
		{
			version:    "1.2.3-dev",
			constraint: "< 1.2.3-dev2",
		},
		{
			version:     "1.2.3-dev",
			constraint:  "< 1.2.3-dev2",
			prereleases: true,
		},
		{
			version:    "0.6.0-rc1",
			constraint: "~> 0.5.0",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "0.6.0-rc1",
			constraint:  "~> 0.5.0",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:    "0.6.0-rc1",
			constraint: "~> 0.6.0-rc1",
		},
		{
			version:     "0.6.0-rc1",
			constraint:  "~> 0.6.0-rc1",
			prereleases: true,
		},
		{
			version:    "0.6.0-rc1",
			constraint: "~> 0.5.0",
			want:       errors.E(versions.ErrCheck),
		},
		{
			version:     "2.0.0-alpha",
			constraint:  "~> 1",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
		{
			version:     "1.0.0-alpha",
			constraint:  "< 2",
			prereleases: true,
		},
		{
			version:    "0.6.0-rc1",
			constraint: "> 0.5, ~> 0.5.0",
			want:       errors.E(versions.ErrCheck),
		},
		// TODO(i4k): review this with Marius.
		// looks broken.
		{
			version:     "1.0.0-alpha",
			constraint:  "< 1.0.0",
			prereleases: true,
			want:        errors.E(versions.ErrCheck),
		},
	} {
		tc := tc
		name := fmt.Sprintf("CheckVersionFor(%q,%q, %t)", tc.version, tc.constraint, tc.prereleases)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := versions.Check(tc.version, tc.constraint, tc.prereleases)
			errtest.Assert(t, err, tc.want, "error mismatch")
		})
	}
}
