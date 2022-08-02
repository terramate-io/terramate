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

package module_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/module"
)

func TestModVendor(t *testing.T) {
}

func TestParseGitSources(t *testing.T) {
	type want struct {
		parsed module.Source
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
				parsed: module.Source{
					Remote: "https://github.com/mineiros-io/example.git",
				},
			},
		},
		{
			name:   "github source with ref",
			source: "github.com/mineiros-io/example?ref=v1",
			want: want{
				parsed: module.Source{
					Remote: "https://github.com/mineiros-io/example.git",
					Ref:    "v1",
				},
			},
		},
		{
			name:   "git@ source",
			source: "git@github.com:mineiros-io/example.git",
			want: want{
				parsed: module.Source{
					Remote: "git@github.com:mineiros-io/example.git",
				},
			},
		},
		{
			name:   "git@ source with ref",
			source: "git@github.com:mineiros-io/example.git?ref=v2",
			want: want{
				parsed: module.Source{
					Remote: "git@github.com:mineiros-io/example.git",
					Ref:    "v2",
				},
			},
		},
		{
			name:   "git::https source",
			source: "git::https://example.com/vpc.git",
			want: want{
				parsed: module.Source{
					Remote: "https://example.com/vpc.git",
				},
			},
		},
		{
			name:   "git::https source with ref",
			source: "git::https://example.com/vpc.git?ref=v3",
			want: want{
				parsed: module.Source{
					Remote: "https://example.com/vpc.git",
					Ref:    "v3",
				},
			},
		},
		{
			name:   "git::ssh source",
			source: "git::ssh://username@example.com/storage.git",
			want: want{
				parsed: module.Source{
					Remote: "ssh://username@example.com/storage.git",
				},
			},
		},
		{
			name:   "git::ssh source with ref",
			source: "git::ssh://username@example.com/storage.git?ref=v4",
			want: want{
				parsed: module.Source{
					Remote: "ssh://username@example.com/storage.git",
					Ref:    "v4",
				},
			},
		},
		{
			name:   "https is not supported",
			source: "https://example.com/vpc-module.zip",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "hg is not supported",
			source: "hg::http://example.com/vpc.hg",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "terraform registry is not supported",
			source: "hashicorp/consul/aws",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "registry is not supported",
			source: "app.terraform.io/example-corp/k8s-cluster/azurerm",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "bitbucket is not supported",
			source: "bitbucket.org/hashicorp/terraform-consul-aws",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "gcs is not supported",
			source: "gcs::https://www.googleapis.com/storage/v1/modules/foomodule.zip",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
		{
			name:   "s3 is not supported",
			source: "s3::https://s3-eu-west-1.amazonaws.com/examplecorp-terraform-modules/vpc.zip",
			want: want{
				err: errors.E(module.ErrUnsupportedModSrc),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			parsed, err := module.ParseSource(tcase.source)
			assert.IsError(t, err, tcase.want.err)
			if tcase.want.err != nil {
				return
			}
			assert.Partial(t, parsed, tcase.want.parsed)
		})
	}
}
