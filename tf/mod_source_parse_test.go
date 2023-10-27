// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tf_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/tf"
)

func TestParseGitSources(t *testing.T) {
	t.Parallel()
	type want struct {
		parsed tf.Source
		err    error
	}

	type testcase struct {
		name   string
		source string
		want   want
	}

	tcases := []testcase{
		{
			name:   "github source",
			source: "github.com/terramate-io/example",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "github source with subdir",
			source: "github.com/terramate-io/example//subdir",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Subdir:     "/subdir",
				},
			},
		},
		{
			name:   "github source with empty subdir",
			source: "github.com/terramate-io/example//",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "github source with .git suffix",
			source: "github.com/terramate-io/example.git",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "github source with .git suffix and subdir",
			source: "github.com/terramate-io/example.git//subdir/dir",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Subdir:     "/subdir/dir",
				},
			},
		},
		{
			name:   "github source with ref",
			source: "github.com/terramate-io/example?ref=v1",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Ref:        "v1",
				},
			},
		},
		{
			name:   "github source with ref and subdir",
			source: "github.com/terramate-io/example//sub/ref?ref=v1",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Subdir:     "/sub/ref",
					Ref:        "v1",
				},
			},
		},
		{
			name:   "github source with ref and .git suffix",
			source: "github.com/terramate-io/example.git?ref=v1",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Ref:        "v1",
				},
			},
		},
		{
			name:   "github source with ref .git suffix and subdir",
			source: "github.com/terramate-io/example.git//dir?ref=v1",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
					Subdir:     "/dir",
					Ref:        "v1",
				},
			},
		},
		{
			name:   "github source with unknown query param ignored",
			source: "github.com/terramate-io/example?key=v1",
			want: want{
				parsed: tf.Source{
					URL:        "https://github.com/terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "git@ source",
			source: "git@github.com:terramate-io/example.git",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
				},
			},
		},
		{
			name:   "git@ source with subdir",
			source: "git@github.com:terramate-io/example.git//subdir",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
					Subdir:     "/subdir",
				},
			},
		},
		{
			name:   "git@ source with empty subdir",
			source: "git@github.com:terramate-io/example.git//",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
				},
			},
		},
		{
			name:   "git@ source with ref",
			source: "git@github.com:terramate-io/example.git?ref=v2",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
					Ref:        "v2",
				},
			},
		},
		{
			name:   "git@ source with ref and subdir",
			source: "git@github.com:terramate-io/example.git//sub/dir?ref=v2",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
					Subdir:     "/sub/dir",
					Ref:        "v2",
				},
			},
		},
		{
			name:   "git@ source with unknown query param ignored",
			source: "git@github.com:terramate-io/example.git?key=v2",
			want: want{
				parsed: tf.Source{
					URL:        "git@github.com:terramate-io/example.git",
					Path:       "github.com/terramate-io/example",
					PathScheme: "git",
				},
			},
		},
		{
			name:   "git::https source",
			source: "git::https://example.com/vpc.git",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "git::https source with subdir",
			source: "git::https://example.com/vpc.git//subdir",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
					Subdir:     "/subdir",
				},
			},
		},
		{
			name:   "git::https source with empty subdir",
			source: "git::https://example.com/vpc.git//",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "git::file:// source with subdir",
			source: "git::file:///tmp/test/repo//subdir",
			want: want{
				parsed: tf.Source{
					URL:        "file:///tmp/test/repo",
					Path:       "/tmp/test/repo",
					PathScheme: "file",
					Subdir:     "/subdir",
				},
			},
		},
		{
			name:   "git::https source with ref",
			source: "git::https://example.com/vpc.git?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
					Ref:        "v3",
				},
			},
		},
		{
			name:   "git::https source with ref and subdir",
			source: "git::https://example.com/vpc.git//sub/dir?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
					Subdir:     "/sub/dir",
					Ref:        "v3",
				},
			},
		},
		{
			name:   "git::https source with port",
			source: "git::https://example.com:443/vpc.git?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com:443/vpc.git",
					Path:       "example.com-443/vpc",
					PathScheme: "https",
					Ref:        "v3",
				},
			},
		},
		{
			name:   "git::https source with port and subdir",
			source: "git::https://example.com:443/vpc.git//port/dir?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com:443/vpc.git",
					Path:       "example.com-443/vpc",
					PathScheme: "https",
					Subdir:     "/port/dir",
					Ref:        "v3",
				},
			},
		},
		{
			name:   "git::https source with unknown query param ignored",
			source: "git::https://example.com/vpc.git?key=v3",
			want: want{
				parsed: tf.Source{
					URL:        "https://example.com/vpc.git",
					Path:       "example.com/vpc",
					PathScheme: "https",
				},
			},
		},
		{
			name:   "git::ssh source",
			source: "git::ssh://username@example.com/storage.git",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
				},
			},
		},
		{
			name:   "git::ssh source and subdir",
			source: "git::ssh://username@example.com/storage.git//subdir",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
					Subdir:     "/subdir",
				},
			},
		},
		{
			name:   "git::ssh source and empty subdir",
			source: "git::ssh://username@example.com/storage.git//",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
				},
			},
		},
		{
			name:   "git::ssh source with port",
			source: "git::ssh://username@example.com:666/storage.git",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com:666/storage.git",
					Path:       "example.com-666/storage",
					PathScheme: "ssh",
				},
			},
		},
		{
			name:   "git::ssh source with port and subdir",
			source: "git::ssh://username@example.com:666/storage.git//ssh/dir",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com:666/storage.git",
					Path:       "example.com-666/storage",
					PathScheme: "ssh",
					Subdir:     "/ssh/dir",
				},
			},
		},
		{
			name:   "git::ssh source with ref",
			source: "git::ssh://username@example.com/storage.git?ref=v4",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
					Ref:        "v4",
				},
			},
		},
		{
			name:   "git::ssh source with ref and subdir",
			source: "git::ssh://username@example.com/storage.git//sub/ref?ref=v4",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
					Subdir:     "/sub/ref",
					Ref:        "v4",
				},
			},
		},
		{
			name:   "git::ssh source with unknown query param ignored",
			source: "git::ssh://username@example.com/storage.git?key=v4",
			want: want{
				parsed: tf.Source{
					URL:        "ssh://username@example.com/storage.git",
					Path:       "example.com/storage",
					PathScheme: "ssh",
				},
			},
		},
		{
			name:   "local is not supported",
			source: "./vpc-module.zip",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "https is not supported",
			source: "https://example.com/vpc-module.zip",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "hg is not supported",
			source: "hg::http://example.com/vpc.hg",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "terraform registry is not supported",
			source: "hashicorp/consul/aws",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "registry is not supported",
			source: "app.terraform.io/example-corp/k8s-cluster/azurerm",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "bitbucket is not supported",
			source: "bitbucket.org/hashicorp/terraform-consul-aws",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "gcs is not supported",
			source: "gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "s3 is not supported",
			source: "s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip",
			want: want{
				err: errors.E(tf.ErrUnsupportedModSrc),
			},
		},
	}

	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			got, err := tf.ParseSource(tcase.source)
			assert.IsError(t, err, tcase.want.err)
			if tcase.want.err != nil {
				return
			}
			tcase.want.parsed.Raw = tcase.source
			test.AssertDiff(t, got, tcase.want.parsed)
		})
	}
}
