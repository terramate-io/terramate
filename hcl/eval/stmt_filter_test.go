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

package eval_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
)

func TestStmtSelectBy(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		ref   eval.Ref
		stmts eval.Stmts
		want  eval.Stmts
		found bool
	}

	for _, tc := range []testcase{
		{
			name: "exact match with origin",
			ref:  eval.Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
			},
			found: true,
		},
		{
			name: "exact match with lhs",
			ref:  eval.Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a"}},
				},
			},
			found: true,
		},
		{
			name: "partial match",
			ref:  eval.Ref{Object: "global", Path: []string{"a"}},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "c"}},
				},
			},
			found: false,
		},
		{
			name: "no match -- in same branch",
			ref:  eval.Ref{Object: "global", Path: []string{"a", "b", "c"}},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
			},
			found: false,
		},
		{
			name: "no match -- in different branch",
			ref:  eval.Ref{Object: "global", Path: []string{"unknown"}},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				eval.Stmt{
					LHS:    eval.Ref{Object: "global", Path: []string{"c"}},
					Origin: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want:  eval.Stmts{},
			found: false,
		},
		{
			name: "root match -- should always return all globals",
			ref:  eval.Ref{Object: "global"},
			stmts: eval.Stmts{
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"a"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"b"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: eval.Stmts{
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"a"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"b"}},
				},
				eval.Stmt{
					LHS: eval.Ref{Object: "global", Path: []string{"c"}},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, found := tc.stmts.SelectBy(tc.ref, map[eval.RefStr]eval.Ref{})
			if found != tc.found {
				t.Fatalf("expected found=%t but got %t", found, tc.found)
			}
			if diff := cmp.Diff(got, tc.want,
				cmp.AllowUnexported(eval.Stmt{}, project.Path{}, info.Range{}, info.Pos{}, hhcl.Range{}),
				cmpopts.IgnoreTypes(cty.Value{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
