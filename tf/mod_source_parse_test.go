// Copyright 2022 Mineiros GmbH
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

package tf_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/tf"
)

func TestParseGitSources(t *testing.T) {
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
			source: "github.com/mineiros-io/example",
			want: want{
				parsed: tf.Source{
					URL:  "https://github.com/mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "github source with ref",
			source: "github.com/mineiros-io/example?ref=v1",
			want: want{
				parsed: tf.Source{
					URL:  "https://github.com/mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
					Ref:  "v1",
				},
			},
		},
		{
			name:   "github source with unknown query param ignored",
			source: "github.com/mineiros-io/example?key=v1",
			want: want{
				parsed: tf.Source{
					URL:  "https://github.com/mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "git@ source",
			source: "git@github.com:mineiros-io/example.git",
			want: want{
				parsed: tf.Source{
					URL:  "git@github.com:mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "git@ source with ref",
			source: "git@github.com:mineiros-io/example.git?ref=v2",
			want: want{
				parsed: tf.Source{
					URL:  "git@github.com:mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
					Ref:  "v2",
				},
			},
		},
		{
			name:   "git@ source with unknown query param ignored",
			source: "git@github.com:mineiros-io/example.git?key=v2",
			want: want{
				parsed: tf.Source{
					URL:  "git@github.com:mineiros-io/example.git",
					Path: "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "git::https source",
			source: "git::https://example.com/vpc.git",
			want: want{
				parsed: tf.Source{
					URL:  "https://example.com/vpc.git",
					Path: "example.com/vpc",
				},
			},
		},
		{
			name:   "git::https source with ref",
			source: "git::https://example.com/vpc.git?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:  "https://example.com/vpc.git",
					Path: "example.com/vpc",
					Ref:  "v3",
				},
			},
		},
		{
			name:   "git::https source with port",
			source: "git::https://example.com:443/vpc.git?ref=v3",
			want: want{
				parsed: tf.Source{
					URL:  "https://example.com:443/vpc.git",
					Path: "example.com-443/vpc",
					Ref:  "v3",
				},
			},
		},
		{
			name:   "git::https source with unknown query param ignored",
			source: "git::https://example.com/vpc.git?key=v3",
			want: want{
				parsed: tf.Source{
					URL:  "https://example.com/vpc.git",
					Path: "example.com/vpc",
				},
			},
		},
		{
			name:   "git::ssh source",
			source: "git::ssh://username@example.com/storage.git",
			want: want{
				parsed: tf.Source{
					URL:  "ssh://username@example.com/storage.git",
					Path: "example.com/storage",
				},
			},
		},
		{
			name:   "git::ssh source with port",
			source: "git::ssh://username@example.com:666/storage.git",
			want: want{
				parsed: tf.Source{
					URL:  "ssh://username@example.com:666/storage.git",
					Path: "example.com-666/storage",
				},
			},
		},
		{
			name:   "git::ssh source with ref",
			source: "git::ssh://username@example.com/storage.git?ref=v4",
			want: want{
				parsed: tf.Source{
					URL:  "ssh://username@example.com/storage.git",
					Path: "example.com/storage",
					Ref:  "v4",
				},
			},
		},
		{
			name:   "git::ssh source with unknown query param ignored",
			source: "git::ssh://username@example.com/storage.git?key=v4",
			want: want{
				parsed: tf.Source{
					URL:  "ssh://username@example.com/storage.git",
					Path: "example.com/storage",
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
		t.Run(tcase.name, func(t *testing.T) {
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
