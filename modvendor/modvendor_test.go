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

package modvendor_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
)

func TestModVendorWithRef(t *testing.T) {
}

func TestModVendorNoRefFails(t *testing.T) {
	// TODO(katcipis): when we start parsing modules for sources
	// we need to address default remote references. for now it is
	// always explicit.
}

func TestParseGitSources(t *testing.T) {
	type want struct {
		parsed modvendor.Source
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
				parsed: modvendor.Source{
					Remote: "https://github.com/mineiros-io/example.git",
					Path:   "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "github source with ref",
			source: "github.com/mineiros-io/example?ref=v1",
			want: want{
				parsed: modvendor.Source{
					Remote: "https://github.com/mineiros-io/example.git",
					Path:   "github.com/mineiros-io/example",
					Ref:    "v1",
				},
			},
		},
		{
			name:   "git@ source",
			source: "git@github.com:mineiros-io/example.git",
			want: want{
				parsed: modvendor.Source{
					Remote: "git@github.com:mineiros-io/example.git",
					Path:   "github.com/mineiros-io/example",
				},
			},
		},
		{
			name:   "git@ source with ref",
			source: "git@github.com:mineiros-io/example.git?ref=v2",
			want: want{
				parsed: modvendor.Source{
					Remote: "git@github.com:mineiros-io/example.git",
					Path:   "github.com/mineiros-io/example",
					Ref:    "v2",
				},
			},
		},
		{
			name:   "git::https source",
			source: "git::https://example.com/vpc.git",
			want: want{
				parsed: modvendor.Source{
					Remote: "https://example.com/vpc.git",
					Path:   "example.com/vpc",
				},
			},
		},
		{
			name:   "git::https source with ref",
			source: "git::https://example.com/vpc.git?ref=v3",
			want: want{
				parsed: modvendor.Source{
					Remote: "https://example.com/vpc.git",
					Path:   "example.com/vpc",
					Ref:    "v3",
				},
			},
		},
		{
			name:   "git::ssh source",
			source: "git::ssh://username@example.com/storage.git",
			want: want{
				parsed: modvendor.Source{
					Remote: "ssh://username@example.com/storage.git",
					Path:   "example.com/storage",
				},
			},
		},
		{
			name:   "git::ssh source with ref",
			source: "git::ssh://username@example.com/storage.git?ref=v4",
			want: want{
				parsed: modvendor.Source{
					Remote: "ssh://username@example.com/storage.git",
					Path:   "example.com/storage",
					Ref:    "v4",
				},
			},
		},
		{
			name:   "fails on missing reference",
			source: "github.com/mineiros-io/example?ref",
			want: want{
				err: errors.E(modvendor.ErrInvalidModSrc),
			},
		},
		{
			name:   "fails on wrong reference",
			source: "github.com/mineiros-io/example?wrong=v1",
			want: want{
				err: errors.E(modvendor.ErrInvalidModSrc),
			},
		},
		{
			name:   "fails on extra unknown params",
			source: "github.com/mineiros-io/example?wrong=v1,ref=v2",
			want: want{
				err: errors.E(modvendor.ErrInvalidModSrc),
			},
		},
		{
			name:   "fails on ? inside ref",
			source: "github.com/mineiros-io/example?ref=v?2",
			want: want{
				err: errors.E(modvendor.ErrInvalidModSrc),
			},
		},
		{
			name:   "fails on empty reference",
			source: "github.com/mineiros-io/example?ref=",
			want: want{
				err: errors.E(modvendor.ErrInvalidModSrc),
			},
		},
		{
			name:   "https is not supported",
			source: "https://example.com/vpc-module.zip",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "hg is not supported",
			source: "hg::http://example.com/vpc.hg",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "terraform registry is not supported",
			source: "hashicorp/consul/aws",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "registry is not supported",
			source: "app.terraform.io/example-corp/k8s-cluster/azurerm",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "bitbucket is not supported",
			source: "bitbucket.org/hashicorp/terraform-consul-aws",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "gcs is not supported",
			source: "gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "s3 is not supported",
			source: "s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip",
			want: want{
				err: errors.E(modvendor.ErrUnsupportedModSrc),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got, err := modvendor.ParseSource(tcase.source)
			assert.IsError(t, err, tcase.want.err)
			if tcase.want.err != nil {
				return
			}
			test.AssertDiff(t, got, tcase.want.parsed)
		})
	}
}
