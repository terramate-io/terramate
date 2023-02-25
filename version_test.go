package terramate_test

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/errors"
	errtest "github.com/mineiros-io/terramate/test/errors"
)

func TestTerramateVersionConstraints(t *testing.T) {
	t.Parallel()

	type testcase struct {
		version    string
		constraint string
		want       error
	}

	for _, tc := range []testcase{
		{
			version:    "0.0.0",
			constraint: "0.0.0",
		},
		{
			version:    "1.2.3",
			constraint: "~> 1.2.3",
		},
		{
			version:    "1.2.3",
			constraint: "~> 1.2",
		},
		{
			version:    "1.2.3",
			constraint: "~> 1",
		},
		{
			version:    "1.2.3",
			constraint: "> 1",
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2",
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2.2",
		},
		{
			version:    "1.2.3",
			constraint: "> 1.2.3",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3",
			constraint: "= 1.2.3",
		},
		{
			version:    "1.2.3",
			constraint: "< 2",
		},
		{
			version:    "1.2.3",
			constraint: "< 1.3",
		},
		{
			version:    "1.2.3",
			constraint: "< 1.2.4",
		},
		{
			version:    "1.2.3",
			constraint: "< 1.2.3",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3",
			constraint: "<= 1.2.3",
		},
		// pre-release are not selected if not present in the constraint
		{
			version:    "1.2.3-dev",
			constraint: "~> 1",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1.2",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-dev",
			constraint: ">= 1.2.3",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-dev",
			constraint: "~> 1.2",
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-dev",
			constraint: "~> 1.2.3",
			want:       errors.E(terramate.ErrVersion),
		},
		// pre-releases are selected if constraint constains pre-release
		{
			version:    "1.2.3-aaa",
			constraint: "~> 1.2.3-aaa", // matches exactly
		},
		{
			version:    "1.2.3-aaa",
			constraint: "> 1.2.3-aab", // not a match because aab > aaa
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-aab",
			constraint: "~> 1.2.3-aaa", // match because aab > aaa
		},
		{
			version:    "1.2.3-zzz",
			constraint: "> 1.2.3-aaa", // match because zzz > aaa
		},
		{
			version:    "1.2.3-zzz",
			constraint: "> 1, > 1.2.3-aaa", // doesnt match because > 1 never matches pre-releases
			want:       errors.E(terramate.ErrVersion),
		},
		{
			version:    "1.2.3-zzz",
			constraint: "> 1.2.3-aaa", // match because aab > aaa
		},
		{
			// matches exactly even if metadata is present
			version:    "1.2.3-dev",
			constraint: "~> 1.2.3-dev+metadata",
		},
		{
			// matches exactly even if metadata is present and different
			version:    "1.2.3-zzz+aaa",
			constraint: ">= 1.2.3-zzz+zzz",
		},
		{
			version:    "1.2.3-dev",
			constraint: "< 1.2.3-dev2",
		},
	} {
		tc := tc
		name := fmt.Sprintf("CheckVersionFor(%q,%q)", tc.version, tc.constraint)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := terramate.CheckVersionFor(tc.version, tc.constraint)
			errtest.Assert(t, err, tc.want, "error mismatch")
		})
	}
}
