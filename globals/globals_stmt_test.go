package globals

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/globals2"
	"github.com/mineiros-io/terramate/project"
)

func TestStmtsLookupRef(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name  string
		ref   globals2.Ref
		stmts globals2.Stmts
		want  globals2.Stmts
		found bool
	}

	for _, tc := range []testcase{
		{
			name: "exact match with origin",
			ref:  globals2.Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
			},
			found: true,
		},
		{
			name: "exact match with lhs",
			ref:  globals2.Ref{Object: "global", Path: []string{"a", "b"}},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a"}},
				},
			},
			found: true,
		},
		{
			name: "partial match",
			ref:  globals2.Ref{Object: "global", Path: []string{"a"}},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "c"}},
				},
			},
			found: false,
		},
		{
			name: "no match -- in same branch",
			ref:  globals2.Ref{Object: "global", Path: []string{"a", "b", "c"}},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want:  globals2.Stmts{},
			found: false,
		},
		{
			name: "no match -- in different branch",
			ref:  globals2.Ref{Object: "global", Path: []string{"unknown"}},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "b"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"a", "c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"a", "c"}},
				},
				globals2.Stmt{
					LHS:    globals2.Ref{Object: "global", Path: []string{"c"}},
					Origin: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want:  globals2.Stmts{},
			found: false,
		},
		{
			name: "root match -- should always return all globals",
			ref:  globals2.Ref{Object: "global"},
			stmts: globals2.Stmts{
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"a"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"b"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
			want: globals2.Stmts{
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"a", "b"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"a"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"b"}},
				},
				globals2.Stmt{
					LHS: globals2.Ref{Object: "global", Path: []string{"c"}},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, found := tc.stmts.SelectBy(tc.ref)
			if found != tc.found {
				t.Fatalf("expected found=%t but got %t", found, tc.found)
			}
			if diff := cmp.Diff(got, tc.want, cmp.AllowUnexported(globals2.Stmt{}, project.Path{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
