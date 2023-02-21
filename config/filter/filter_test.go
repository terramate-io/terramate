// Copyright 2023 Mineiros GmbH
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

package filter

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config/tag"
	"github.com/mineiros-io/terramate/errors"
	errtest "github.com/mineiros-io/terramate/test/errors"

	"github.com/google/go-cmp/cmp"
)

func TestFilterParserTags(t *testing.T) {
	t.Parallel()

	type testcase struct {
		filters   []string
		want      TagClause
		noClauses bool
		err       error
	}

	for _, tc := range []testcase{
		{
			filters: []string{
				"a",
			},
			want: TagClause{
				Op:  EQ,
				Tag: "a",
			},
		},
		{
			filters: []string{
				"~a",
			},
			want: TagClause{
				Op:  NEQ,
				Tag: "a",
			},
		},
		{
			filters: []string{
				"a,b",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  EQ,
						Tag: "a",
					},
					{
						Op:  EQ,
						Tag: "b",
					},
				},
			},
		},
		{
			filters: []string{
				"~a,~b",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  NEQ,
						Tag: "a",
					},
					{
						Op:  NEQ,
						Tag: "b",
					},
				},
			},
		},
		{
			filters: []string{
				"a,b:c",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  EQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
				},
			},
		},
		{
			filters: []string{
				"~a,~b:~c",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  NEQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  NEQ,
								Tag: "b",
							},
							{
								Op:  NEQ,
								Tag: "c",
							},
						},
					},
				},
			},
		},
		{
			filters: []string{
				"a,b:c,d",
			},
			want: TagClause{

				Op: OR,
				Children: []TagClause{
					{
						Op:  EQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
					{
						Op:  EQ,
						Tag: "d",
					},
				},
			},
		},
		{
			filters: []string{
				"~a,b:c,~d",
			},
			want: TagClause{

				Op: OR,
				Children: []TagClause{
					{
						Op:  NEQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
					{
						Op:  NEQ,
						Tag: "d",
					},
				},
			},
		},
		{
			filters: []string{
				"a:b:c,d:e:f,g:h:i",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "a",
							},
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "d",
							},
							{
								Op:  EQ,
								Tag: "e",
							},
							{
								Op:  EQ,
								Tag: "f",
							},
						},
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "g",
							},
							{
								Op:  EQ,
								Tag: "h",
							},
							{
								Op:  EQ,
								Tag: "i",
							},
						},
					},
				},
			},
		},
		{
			filters: []string{
				"a,b:c,d:e",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  EQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "d",
							},
							{
								Op:  EQ,
								Tag: "e",
							},
						},
					},
				},
			},
		},
		{
			filters: []string{
				"a,b:c,d:e:f",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op:  EQ,
						Tag: "a",
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
						},
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "d",
							},
							{
								Op:  EQ,
								Tag: "e",
							},
							{
								Op:  EQ,
								Tag: "f",
							},
						},
					},
				},
			},
		},
		{
			filters: []string{
				"a:b:c:d,e:f:g:h",
			},
			want: TagClause{
				Op: OR,
				Children: []TagClause{
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "a",
							},
							{
								Op:  EQ,
								Tag: "b",
							},
							{
								Op:  EQ,
								Tag: "c",
							},
							{
								Op:  EQ,
								Tag: "d",
							},
						},
					},
					{
						Op: AND,
						Children: []TagClause{
							{
								Op:  EQ,
								Tag: "e",
							},
							{
								Op:  EQ,
								Tag: "f",
							},
							{
								Op:  EQ,
								Tag: "g",
							},
							{
								Op:  EQ,
								Tag: "h",
							},
						},
					},
				},
			},
		},
		{
			filters:   []string{""},
			noClauses: true,
		},
		{
			filters: []string{"_invalid"},
			err:     errors.E(tag.ErrInvalidTag),
		},
		{
			filters: []string{"valid:othervalid,_invalid"},
			err:     errors.E(tag.ErrInvalidTag),
		},
		{
			filters: []string{"valid:_invalid,validagain"},
			err:     errors.E(tag.ErrInvalidTag),
		},
	} {
		t.Run(fmt.Sprintf("filters:%v", tc.filters), func(t *testing.T) {
			got, hasClauses, err := ParseTagClauses(tc.filters...)
			errtest.Assert(t, err, tc.err)
			if !hasClauses != tc.noClauses {
				t.Fatalf("filter emptiness mismatch: %t != %t", !hasClauses, tc.noClauses)
			}
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Fatalf("got[-], want[+], diff = %s", diff)
			}
		})
	}
}

func TestFilterMatchTags(t *testing.T) {
	t.Parallel()

	type testcase struct {
		filters []string
		target  []string // target tags
		want    bool
	}

	for _, tc := range []testcase{
		{
			target: []string{"a", "b"},
			filters: []string{
				"a",
			},
			want: true,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~a",
			},
			want: false,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~a,b",
			},
			want: true,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"a,~b",
			},
			want: true,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~a,~b",
			},
			want: false,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~c",
			},
			want: true,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~c",
			},
			want: true,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"~c:~a",
			},
			want: false,
		},
		{
			target: []string{"a", "b"},
			filters: []string{
				"a:b:~a",
			},
			want: false,
		},
	} {
		name := fmt.Sprintf("test if filters:%v match:%v", tc.filters, tc.target)
		t.Run(name, func(t *testing.T) {
			clauses, _, err := ParseTagClauses(tc.filters...)
			assert.NoError(t, err) // only valid clauses tested here
			res := MatchTags(clauses, tc.target)
			assert.IsTrue(t, res == tc.want,
				"filter %v doesnt match tags %v", tc.filters, tc.target)
		})
	}
}
