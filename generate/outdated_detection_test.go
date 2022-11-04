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
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

func TestOutdatedDetection(t *testing.T) {
	t.Parallel()

	type (
		file struct {
			path string
			body fmt.Stringer
		}
		step struct {
			layout  []string
			files   []file
			want    []string
			wantErr error
		}
		testcase struct {
			name  string
			steps []step
		}
	)

	tcases := []testcase{
		{
			name: "empty project",
			steps: []step{
				{
					want: []string{},
				},
			},
		},
		{
			name: "project with no stacks",
			steps: []step{
				{
					layout: []string{
						"d:emptydir",
						"f:dir/file",
						"f:dir2/file",
					},
					want: []string{},
				},
			},
		},
		{
			name: "generate blocks with no code generated and one stack",
			steps: []step{
				{
					layout: []string{
						"s:stack",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack/test.hcl",
						"stack/test.txt",
					},
				},
			},
		},
		{
			name: "generate blocks with no code generated and multiple stacks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "generate blocks content changed",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "changed"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "changed"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "changed"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "generate_hcl is detected on ex stack",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "stack-2/" + stack.DefaultFilename,
							body: Doc(),
						},
					},
					want: []string{
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			// TODO(KATCIPIS): when we remove the origin from gen code header
			// this behavior will change.
			name: "moving generate blocks to different files is detected on generate_hcl",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
						{
							path: "generate_file.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
							),
						},
						{
							path: "generate_hcl.tm",
							body: Doc(
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "generate_file is not detected when deleted",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
					},
					want: []string{},
				},
			},
		},
		{
			name: "generate_hcl is detected when deleted",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "generate blocks shifting condition",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "multiple generate blocks with same label",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
							),
						},
					},
					want: []string{},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
							),
						},
					},
					want: []string{},
				},
			},
		},
		{
			name: "ignores outdated code on skipped dir",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
						"f:stack-2/" + config.SkipFilename,
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "code"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
					},
				},
			},
		},
		{
			name: "detection on substacks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-1/child",
						"s:stack-2",
						"s:stack-2/dir/child",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								Globals(
									Bool("condition", true),
								),
								GenerateFile(
									Labels("test.txt"),
									Expr("condition", "global.condition"),
									Str("content", "code"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Expr("condition", "global.condition"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/child/test.hcl",
						"stack-1/child/test.txt",
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/dir/child/test.hcl",
						"stack-2/dir/child/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "stack-1/child/config.tm",
							body: Doc(
								Globals(
									Bool("condition", false),
								),
							),
						},
						{
							path: "stack-2/dir/child/config.tm",
							body: Doc(
								Globals(
									Bool("condition", false),
								),
							),
						},
					},
					want: []string{
						"stack-1/child/test.hcl",
						"stack-1/child/test.txt",
						"stack-2/dir/child/test.hcl",
						"stack-2/dir/child/test.txt",
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc

		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)

			for i, step := range tcase.steps {
				t.Logf("step %d", i)

				s.BuildTree(step.layout)
				root := s.RootEntry()

				for _, file := range step.files {
					root.CreateFile(file.path, file.body.String())
				}

				s.ReloadConfig()

				got, err := generate.DetectOutdated(s.Config())

				assert.IsError(t, err, step.wantErr)
				if err != nil {
					continue
				}

				assertEqualStringList(t, got, step.want)

				s.Generate()

				s.ReloadConfig()

				got, err = generate.DetectOutdated(s.Config())
				assert.NoError(t, err)
				assertEqualStringList(t, got, []string{})
			}
		})
	}
}
