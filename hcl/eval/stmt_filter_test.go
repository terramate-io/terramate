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

package eval

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
)

func TestStmtsLookupRef(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		ref   Ref
		stmts Stmts
		want  Stmts
		found bool
	}

	for _, tc := range []testcase{
		{
			name: "exact match with origin",
			ref:  Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"c"}},
					Origin: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
			},
			found: true,
		},
		{
			name: "exact match with lhs",
			ref:  Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"c"}},
					Origin: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a"}},
				},
			},
			found: true,
		},
		{
			name: "partial match",
			ref:  Ref{Object: "global", Path: []string{"a"}},
			stmts: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: Ref{Object: "global", Path: []string{"a", "c"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"c"}},
					Origin: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: Ref{Object: "global", Path: []string{"a", "c"}},
				},
			},
			found: false,
		},
		{
			name: "no match -- in same branch",
			ref:  Ref{Object: "global", Path: []string{"a", "b", "c"}},
			stmts: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: Ref{Object: "global", Path: []string{"a", "c"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"c"}},
					Origin: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
			},
			found: false,
		},
		{
			name: "no match -- in different branch",
			ref:  Ref{Object: "global", Path: []string{"unknown"}},
			stmts: Stmts{
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: Ref{Object: "global", Path: []string{"a", "c"}},
				},
				Stmt{
					LHS:    Ref{Object: "global", Path: []string{"c"}},
					Origin: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want:  Stmts{},
			found: false,
		},
		{
			name: "root match -- should always return all globals",
			ref:  Ref{Object: "global"},
			stmts: Stmts{
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"a"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"b"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: Stmts{
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"a", "b"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"a"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"b"}},
				},
				Stmt{
					LHS: Ref{Object: "global", Path: []string{"c"}},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, found := tc.stmts.SelectBy(tc.ref, map[RefStr]Ref{})
			if found != tc.found {
				t.Fatalf("expected found=%t but got %t", found, tc.found)
			}
			if diff := cmp.Diff(got, tc.want,
				cmp.AllowUnexported(Stmt{}, project.Path{}, info.Range{}, info.Pos{}, hhcl.Range{}),
				cmpopts.IgnoreTypes(cty.Value{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
