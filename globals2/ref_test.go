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

package globals2

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/project"
)

func TestRefsOf(t *testing.T) {
	t.Parallel()

	type testcase struct {
		expr string
		want []Ref
	}

	for _, tc := range []testcase{
		{
			expr: `global.a`,
			want: []Ref{
				{Object: "global", Path: []string{"a"}},
			},
		},
		{
			expr: `global.a.b.c + global.x.y.z`,
			want: []Ref{
				{Object: "global", Path: []string{"a", "b", "c"}},
				{Object: "global", Path: []string{"x", "y", "z"}},
			},
		},
		{
			expr: `global.a + global.a * global.a`,
			want: []Ref{
				// unique
				{Object: "global", Path: []string{"a"}},
			},
		},
		{
			expr: `global["a"]`,
			want: []Ref{
				{Object: "global", Path: []string{"a"}},
			},
		},
		{
			expr: `global["a"]["b"]`,
			want: []Ref{
				{Object: "global", Path: []string{"a", "b"}},
			},
		},
		{
			expr: `global["a"][global.b]`,
			want: []Ref{
				{Object: "global", Path: []string{"a"}},
				{Object: "global", Path: []string{"b"}},
			},
		},
		{
			expr: `global[global]`,
			want: []Ref{
				{Object: "global"},
			},
		},
		{
			expr: `{
				a = global.a
				b = {
					c = {
						d = {
							e = {
								f = global.b
							}
						}
					}
				}
			}`,
			want: []Ref{
				{Object: "global", Path: []string{"a"}},
				{Object: "global", Path: []string{"b"}},
			},
		},
		{
			expr: `tm_call(global.a)+tm_call(tm_other(tm_bleh(tm_a(hidden.thing))))`,
			want: []Ref{
				{Object: "global", Path: []string{"a"}},
			},
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("refsOf(%s)", tc.expr), func(t *testing.T) {
			expr, diags := hclsyntax.ParseExpression([]byte(tc.expr), "test.hcl", hhcl.InitialPos)
			if diags.HasErrors() {
				t.Fatal(diags.Error())
			}
			refs := refsOf(expr)
			if !refs.equal(tc.want) {
				t.Fatalf(fmt.Sprintf("(%v != %v)", refs, tc.want))
			}
		})
	}
}

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
			want:  Stmts{},
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
			got, found := tc.stmts.selectBy(tc.ref)
			if found != tc.found {
				t.Fatalf("expected found=%t but got %t", found, tc.found)
			}
			if diff := cmp.Diff(got, tc.want, cmp.AllowUnexported(Stmt{}, project.Path{})); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestRefEquals(t *testing.T) {
	t.Parallel()
	type testcase struct {
		a, b Ref
		want bool
	}

	for _, tc := range []testcase{
		{
			a:    Ref{Object: "global"},
			b:    Ref{Object: "terramate"},
			want: false,
		},
		{
			a:    Ref{Object: "global"},
			b:    Ref{Object: "global"},
			want: true,
		},
		{
			a:    Ref{Object: "global", Path: nil},
			b:    Ref{Object: "global", Path: []string{}},
			want: true,
		},
		{
			a:    Ref{Object: "global", Path: []string{"a"}},
			b:    Ref{Object: "global", Path: []string{}},
			want: false,
		},
		{
			a:    Ref{Object: "global", Path: []string{"a", "b"}},
			b:    Ref{Object: "global", Path: []string{"a", "b"}},
			want: true,
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("%s == %s", tc.a, tc.b), func(t *testing.T) {
			if tc.a.equal(tc.b) != tc.want {
				t.Fatalf(fmt.Sprintf("(%s == %s) != %t", tc.a, tc.b, tc.want))
			}
		})
	}
}

func TestRefString(t *testing.T) {
	t.Parallel()
	type testcase struct {
		in   Ref
		want string
	}

	for _, tc := range []testcase{
		{
			in:   Ref{Object: "global"},
			want: `global`,
		},
		{
			in:   Ref{Object: "global", Path: []string{"a", "b"}},
			want: `global["a"]["b"]`,
		},
		{
			in:   Ref{Object: "global", Path: []string{"spaces and\nnew lines"}},
			want: "global[\"spaces and\\nnew lines\"]",
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("object:%s, path:%v", tc.in.Object, tc.in.Path), func(t *testing.T) {
			assert.EqualStrings(t, tc.want, tc.in.String())
		})
	}
}
