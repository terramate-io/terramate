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

package generate_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclutils"
	. "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoad(t *testing.T) {
	type (
		file struct {
			path string
			body fmt.Stringer
		}
		genfile struct {
			label      string
			blockRange info.Range
			condition  bool
		}
		result struct {
			dir   string
			files []genfile
			err   error
		}
		testcase struct {
			name    string
			layout  []string
			configs []file
			want    []result
			wantErr error
		}
	)
	t.Parallel()

	tcases := []testcase{
		{
			name: "no stacks",
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
			},
			want: []result{},
		},
		{
			name: "no generate blocks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			want: []result{
				{
					dir: "/stack-1",
				},
				{
					dir: "/stack-2",
				},
			},
		},
		{
			name: "generate blocks range information",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
				"d:modules",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
						Import(
							Str("source", "modules/imported.tm"),
						),
					),
				},
				{
					path: "modules/imported.tm",
					body: Doc(
						GenerateHCL(
							Labels("test2.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test2.txt"),
							Str("content", "test"),
						),
					),
				},
			},
			want: []result{
				{
					dir: "/stack-1",
					files: []genfile{
						{
							label:     "test.hcl",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(1, 1, 0),
								End(5, 2, 63),
							),
						},
						{
							label:     "test.txt",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(6, 1, 64),
								End(8, 2, 111),
							),
						},
						{
							label:     "test2.hcl",
							condition: true,
							blockRange: Range(
								"/modules/imported.tm",
								Start(1, 1, 0),
								End(5, 2, 64),
							),
						},
						{
							label:     "test2.txt",
							condition: true,
							blockRange: Range(
								"/modules/imported.tm",
								Start(6, 1, 65),
								End(8, 2, 113),
							),
						},
					},
				},
				{
					dir: "/stack-2",
					files: []genfile{
						{
							label:     "test.hcl",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(1, 1, 0),
								End(5, 2, 63),
							),
						},
						{
							label:     "test.txt",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(6, 1, 64),
								End(8, 2, 111),
							),
						},
						{
							label:     "test2.hcl",
							condition: true,
							blockRange: Range(
								"/modules/imported.tm",
								Start(1, 1, 0),
								End(5, 2, 64),
							),
						},
						{
							label:     "test2.txt",
							condition: true,
							blockRange: Range(
								"/modules/imported.tm",
								Start(6, 1, 65),
								End(8, 2, 113),
							),
						},
					},
				},
			},
		},
		{
			name: "multiple generate blocks",
			layout: []string{
				"s:stack-1",
				"s:stack-1/child",
				"s:stack-2",
				"s:stack-2/dir/child",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateHCL(
							Labels("test.hcl"),
							Bool("condition", false),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
						GenerateFile(
							Labels("test.txt"),
							Bool("condition", false),
							Str("content", "test"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("stack-1.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
					),
				},
				{
					path: "stack-2/dir/child/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("stack-2-child.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
					),
				},
			},
			want: []result{
				{
					dir: "/stack-1",
					files: []genfile{
						{
							label:     "stack-1.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: true,
						},
					},
				},
				{
					dir: "/stack-1/child",
					files: []genfile{
						{
							label:     "stack-1.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: true,
						},
					},
				},
				{
					dir: "/stack-2",
					files: []genfile{
						{
							label:     "test.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: true,
						},
					},
				},
				{
					dir: "/stack-2/dir/child",
					files: []genfile{
						{
							label:     "stack-2-child.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: true,
						},
						{
							label:     "test.hcl",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: false,
						},
						{
							label:     "test.txt",
							condition: true,
						},
					},
				},
			},
		},
		{
			name: "partial result failing to list stacks",
			layout: []string{
				"s:stack-1:id=duplicated",
				"s:stack-2:id=duplicated",
			},
			wantErr: errors.E(config.ErrStackDuplicatedID),
		},
		{
			name: "partial result failing to load globals",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
				"s:stack-3",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						Globals(
							Expr("a", "global.undefined"),
						),
					),
				},
				{
					path: "stack-2/config.tm",
					body: Doc(
						Globals(
							Expr("a", "global.undefined"),
						),
					),
				},
			},
			want: []result{
				{
					dir: "/stack-1",
					err: errors.E(globals.ErrEval),
				},
				{
					dir: "/stack-2",
					err: errors.E(globals.ErrEval),
				},
				{
					dir: "/stack-3",
					files: []genfile{
						{
							label:     "test.hcl",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(1, 1, 0),
								End(5, 2, 63),
							),
						},
						{
							label:     "test.txt",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(6, 1, 64),
								End(8, 2, 111),
							),
						},
					},
				},
			},
		},
		{
			name: "partial result failing to generate code",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
				"s:stack-3",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Str("stacks", "test"),
							),
						),
						GenerateFile(
							Labels("test.txt"),
							Str("content", "test"),
						),
					),
				},
				{
					path: "stack-2/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Expr("stacks", "global.undefined"),
							),
						),
					),
				},
				{
					path: "stack-3/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("test.hcl"),
							Content(
								Expr("stacks", "global.undefined"),
							),
						),
					),
				},
			},
			want: []result{
				{
					dir: "/stack-1",
					files: []genfile{
						{
							label:     "test.hcl",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(1, 1, 0),
								End(5, 2, 63),
							),
						},
						{
							label:     "test.txt",
							condition: true,
							blockRange: Range(
								"/config.tm",
								Start(6, 1, 64),
								End(8, 2, 111),
							),
						},
					},
				},
				{
					dir: "/stack-2",
					err: errors.E(eval.ErrPartial),
				},
				{
					dir: "/stack-3",
					err: errors.E(eval.ErrPartial),
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)
			root := s.RootEntry()
			for _, cfg := range tcase.configs {
				root.CreateFile(cfg.path, cfg.body.String())
			}

			got, err := generate.Load(s.Config(), project.NewPath("/modules"))
			assert.IsError(t, err, tcase.wantErr)
			if tcase.wantErr != nil {
				return
			}

			if len(got) != len(tcase.want) {
				t.Errorf("got %d results, want %d", len(got), len(tcase.want))
				t.Errorf("got = %v", got)
				t.Fatalf("want = %v", tcase.want)
			}

			for i, got := range got {
				t.Logf("checking result %d", i)

				want := tcase.want[i]
				assert.IsError(t, got.Err, want.err)
				if want.err != nil {
					continue
				}
				wantDir := project.NewPath(want.dir)
				test.AssertEqualPaths(t, wantDir, got.Dir, "dir mismatch")

				if len(got.Files) != len(want.files) {
					t.Errorf("got %d results, want %d", len(got.Files), len(want.files))
					t.Errorf("got = %v", got.Files)
					t.Fatalf("want = %v", want.files)
				}

				for j, gotFile := range got.Files {
					t.Logf("checking result %d file %d", i, j)

					wantFile := want.files[j]

					assert.EqualStrings(t, wantFile.label, gotFile.Label(), "label mismatch")
					assert.IsTrue(t, wantFile.condition == gotFile.Condition(),
						"want condition %t != %t", wantFile.condition, gotFile.Condition())

					wantRange := FixRange(s.RootDir(), wantFile.blockRange)
					test.AssertEqualRanges(t, gotFile.Range(), wantRange)
				}
			}
		})
	}
}
