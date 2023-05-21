// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package filter

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config/tag"
	"github.com/terramate-io/terramate/errors"
	errtest "github.com/terramate-io/terramate/test/errors"

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
			got, hasClauses, err := parseInternalTagClauses(tc.filters...)
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
			clauses, _, err := parseInternalTagClauses(tc.filters...)
			assert.NoError(t, err) // only valid clauses tested here
			res := MatchTags(clauses, tc.target)
			assert.IsTrue(t, res == tc.want,
				"filter %v doesnt match tags %v", tc.filters, tc.target)
		})
	}
}
