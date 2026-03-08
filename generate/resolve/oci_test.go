// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/generate/resolve"
)

func TestIsOCISource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		want bool
	}{
		{"oci://ghcr.io/org/repo:v1.0.0", true},
		{"oci://123456.dkr.ecr.us-west-2.amazonaws.com/bundle:v1", true},
		{"oci://localhost:5000/test:latest", true},
		{"oci://registry.example.com/a/b/c:v2", true},
		{"github.com/org/repo", false},
		{"git::https://github.com/org/repo", false},
		{"/local/path", false},
		{"./relative/path", false},
		{"", false},
		{"oci", false},
		{"oci:", false},
		{"oci:/", false},
		{"OCI://upper.case/repo:v1", false},
	}

	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			got := resolve.IsOCISource(tc.src)
			if got != tc.want {
				t.Errorf("IsOCISource(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

func TestParseOCIReference(t *testing.T) {
	t.Parallel()

	type want struct {
		registry   string
		repository string
		tag        string
		digest     string
		wantErr    bool
	}

	tests := []struct {
		src  string
		want want
	}{
		{
			src: "oci://ghcr.io/org/repo:v1.0.0",
			want: want{
				registry:   "ghcr.io",
				repository: "org/repo",
				tag:        "v1.0.0",
			},
		},
		{
			src: "oci://ghcr.io/org/repo",
			want: want{
				registry:   "ghcr.io",
				repository: "org/repo",
				tag:        "latest",
			},
		},
		{
			src: "oci://ghcr.io/org/repo@sha256:abcdef1234567890",
			want: want{
				registry:   "ghcr.io",
				repository: "org/repo",
				digest:     "sha256:abcdef1234567890",
			},
		},
		{
			src: "oci://123456789.dkr.ecr.us-west-2.amazonaws.com/my-bundle:v2.0.0",
			want: want{
				registry:   "123456789.dkr.ecr.us-west-2.amazonaws.com",
				repository: "my-bundle",
				tag:        "v2.0.0",
			},
		},
		{
			src: "oci://123456789.dkr.ecr.us-west-2.amazonaws.com/bundles/data-plane-aws:v2.0.0",
			want: want{
				registry:   "123456789.dkr.ecr.us-west-2.amazonaws.com",
				repository: "bundles/data-plane-aws",
				tag:        "v2.0.0",
			},
		},
		{
			src: "oci://localhost:5000/test:latest",
			want: want{
				registry:   "localhost:5000",
				repository: "test",
				tag:        "latest",
			},
		},
		{
			src: "oci://registry.example.com/a/b/c:v2",
			want: want{
				registry:   "registry.example.com",
				repository: "a/b/c",
				tag:        "v2",
			},
		},
		{
			src:  "oci://",
			want: want{wantErr: true},
		},
		{
			src:  "oci://registryonly",
			want: want{wantErr: true},
		},
		{
			src:  "oci://registry/",
			want: want{wantErr: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			ref, err := resolve.ParseOCIReference(tc.src)
			if tc.want.wantErr {
				if err == nil {
					t.Fatalf("ParseOCIReference(%q) expected error, got nil", tc.src)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOCIReference(%q) unexpected error: %v", tc.src, err)
			}
			assert.EqualStrings(t, tc.want.registry, ref.Registry)
			assert.EqualStrings(t, tc.want.repository, ref.Repository)
			assert.EqualStrings(t, tc.want.tag, ref.Tag)
			assert.EqualStrings(t, tc.want.digest, ref.Digest)
		})
	}
}

func TestOCIReferenceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref  resolve.OCIReference
		want string
	}{
		{
			ref:  resolve.OCIReference{Registry: "ghcr.io", Repository: "org/repo", Tag: "v1.0.0"},
			want: "ghcr.io/org/repo:v1.0.0",
		},
		{
			ref:  resolve.OCIReference{Registry: "ghcr.io", Repository: "org/repo", Digest: "sha256:abc123"},
			want: "ghcr.io/org/repo@sha256:abc123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.ref.Reference()
			assert.EqualStrings(t, tc.want, got)
		})
	}
}

func TestCombineSourcesOCI(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src       string
		parentSrc string
		want      string
	}

	tests := []testcase{
		// rel src + OCI parent
		{
			src:       "./child",
			parentSrc: "oci://ghcr.io/org/bundle:v1",
			want:      "oci://ghcr.io/org/bundle:v1//child",
		},
		{
			src:       "./child",
			parentSrc: "oci://ghcr.io/org/bundle:v1//subdir",
			want:      "oci://ghcr.io/org/bundle:v1//subdir/child",
		},
		{
			src:       "..",
			parentSrc: "oci://ghcr.io/org/bundle:v1//subdir/deep",
			want:      "oci://ghcr.io/org/bundle:v1//subdir",
		},
		{
			src:       "../sibling",
			parentSrc: "oci://ghcr.io/org/bundle:v1//components/network",
			want:      "oci://ghcr.io/org/bundle:v1//components/sibling",
		},
		// abs src + OCI parent
		{
			src:       "/override",
			parentSrc: "oci://ghcr.io/org/bundle:v1//subdir",
			want:      "oci://ghcr.io/org/bundle:v1//override",
		},
		// OCI src + any parent (OCI URL discards parent)
		{
			src:       "oci://ghcr.io/org/new:v2",
			parentSrc: "oci://ghcr.io/org/old:v1",
			want:      "oci://ghcr.io/org/new:v2",
		},
		{
			src:       "oci://ghcr.io/org/new:v2",
			parentSrc: "./relative",
			want:      "oci://ghcr.io/org/new:v2",
		},
		{
			src:       "oci://ghcr.io/org/new:v2",
			parentSrc: "/absolute",
			want:      "oci://ghcr.io/org/new:v2",
		},
		// rel src + OCI parent (no subdir, should add subdir)
		{
			src:       ".",
			parentSrc: "oci://ghcr.io/org/bundle:v1",
			want:      "oci://ghcr.io/org/bundle:v1",
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s + %s", tc.src, tc.parentSrc), func(t *testing.T) {
			got := resolve.CombineSources(tc.src, tc.parentSrc)
			assert.EqualStrings(t, tc.want, got)
		})
	}
}
