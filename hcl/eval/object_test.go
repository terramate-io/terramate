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
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"

	errtest "github.com/mineiros-io/terramate/test/errors"
)

func TestCtyObjectSetAt(t *testing.T) {
	type testcase struct {
		name    string
		obj     *eval.Object
		val     interface{}
		path    string
		want    *eval.Object
		wantErr error
	}

	for _, tc := range []testcase{
		{
			name: "set at root, empty object",
			obj:  eval.NewObject(),
			path: "key",
			val:  "value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key": "value",
				},
			},
		},
		{
			name: "set at root, override value",
			obj: &eval.Object{
				Keys: map[string]interface{}{
					"key": "old value",
				},
			},
			path: "key",
			val:  "new value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key": "new value",
				},
			},
		},
		{
			name: "set at root, new value",
			obj: &eval.Object{
				Keys: map[string]interface{}{
					"key": "value",
				},
			},
			path: "other-key",
			val:  "other value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key":       "value",
					"other-key": "other value",
				},
			},
		},
		{
			name: "set at an existing child object",
			obj: &eval.Object{
				Keys: map[string]interface{}{
					"key": eval.NewObject(),
				},
			},
			path: "key.test",
			val:  "child value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key": &eval.Object{
						Keys: map[string]interface{}{
							"test": "child value",
						},
					},
				},
			},
		},
		{
			name: "set at an existing child object",
			obj: &eval.Object{
				Keys: map[string]interface{}{
					"key": eval.NewObject(),
				},
			},
			path: "key.test",
			val:  "child value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key": &eval.Object{
						Keys: map[string]interface{}{
							"test": "child value",
						},
					},
				},
			},
		},
		{
			name: "set at a non-existent child object",
			obj:  eval.NewObject(),
			path: "key.test",
			val:  "child value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"key": &eval.Object{
						Keys: map[string]interface{}{
							"test": "child value",
						},
					},
				},
			},
		},
		{
			name: "set at a non-existent deep child object",
			obj:  eval.NewObject(),
			path: "a.b.c.d.test",
			val:  "value",
			want: &eval.Object{
				Keys: map[string]interface{}{
					"a": &eval.Object{
						Keys: map[string]interface{}{
							"b": &eval.Object{
								Keys: map[string]interface{}{
									"c": &eval.Object{
										Keys: map[string]interface{}{
											"d": &eval.Object{
												Keys: map[string]interface{}{
													"test": "value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "set at a non-object child - fails",
			obj: &eval.Object{
				Keys: map[string]interface{}{
					"key": 1,
				},
			},
			path:    "key.test",
			val:     "child value",
			wantErr: errors.E(eval.ErrCannotExtendObject),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.obj.SetAt(eval.DotPath(tc.path), tc.val)
			errtest.Assert(t, err, tc.wantErr)
			if err == nil {
				if diff := cmp.Diff(tc.obj, tc.want); diff != "" {
					t.Fatalf("-(got) +(want):\n%s", diff)
				}
			}
		})
	}
}
