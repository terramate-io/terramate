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

package eval_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"

	errtest "github.com/mineiros-io/terramate/test/errors"
)

type strValue string

func (s strValue) IsObject() bool       { return false }
func (s strValue) Origin() project.Path { return project.NewPath("/") }

func TestCtyObjectSetAt(t *testing.T) {
	type testcase struct {
		name    string
		obj     *eval.Object
		val     eval.Value
		path    string
		want    *eval.Object
		wantErr error
	}

	newobj := func(sets ...map[string]eval.Value) *eval.Object {
		obj := eval.NewObject(project.NewPath("/"))
		for _, set := range sets {
			obj.SetFrom(set)
		}
		return obj
	}

	for _, tc := range []testcase{
		{
			name: "set at root, empty object",
			obj:  newobj(),
			path: "key",
			val:  strValue("value"),
			want: newobj(map[string]eval.Value{
				"key": strValue("value"),
			}),
		},
		{
			name: "set at root, override value",
			obj: newobj(map[string]eval.Value{
				"key": strValue("old value"),
			}),
			path: "key",
			val:  strValue("new value"),
			want: newobj().SetFrom(
				map[string]eval.Value{
					"key": strValue("new value"),
				},
			),
		},
		{
			name: "set at root, new value",
			obj: newobj(map[string]eval.Value{
				"key": strValue("value"),
			},
			),
			path: "other-key",
			val:  strValue("other value"),
			want: newobj(map[string]eval.Value{
				"key":       strValue("value"),
				"other-key": strValue("other value"),
			}),
		},
		{
			name: "set at an existing child object",
			obj: newobj(map[string]eval.Value{
				"key": newobj(),
			}),
			path: "key.test",
			val:  strValue("child value"),
			want: newobj(map[string]eval.Value{
				"key": newobj(map[string]eval.Value{
					"test": strValue("child value"),
				}),
			}),
		},
		{
			name: "set at an existing child object",
			obj: newobj(map[string]eval.Value{
				"key": newobj(),
			}),
			path: "key.test",
			val:  strValue("child value"),
			want: newobj(map[string]eval.Value{
				"key": newobj(map[string]eval.Value{
					"test": strValue("child value"),
				}),
			}),
		},
		{
			name: "set at a non-existent child object",
			obj:  newobj(),
			path: "key.test",
			val:  strValue("child value"),
			want: newobj(map[string]eval.Value{
				"key": newobj(map[string]eval.Value{
					"test": strValue("child value"),
				}),
			}),
		},
		{
			name: "set at a non-existent deep child object",
			obj:  newobj(),
			path: "a.b.c.d.test",
			val:  strValue("value"),
			want: newobj(map[string]eval.Value{
				"a": newobj(map[string]eval.Value{
					"b": newobj(map[string]eval.Value{
						"c": newobj(map[string]eval.Value{
							"d": newobj(map[string]eval.Value{
								"test": strValue("value"),
							}),
						}),
					}),
				}),
			}),
		},
		{
			name: "set at a non-object child - fails",
			obj: newobj(map[string]eval.Value{
				"key": strValue("1"),
			}),
			path:    "key.test",
			val:     strValue("child value"),
			wantErr: errors.E(eval.ErrCannotExtendObject),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.obj.SetAt(eval.DotPath(tc.path), tc.val)
			errtest.Assert(t, err, tc.wantErr)
			if err == nil {
				if diff := cmp.Diff(tc.obj, tc.want, cmpopts.IgnoreUnexported(eval.Object{})); diff != "" {
					t.Fatalf("-(got) +(want):\n%s", diff)
				}
			}
		})
	}
}
