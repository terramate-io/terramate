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
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
)

// Tests Inside stacks
// - Blocks with same label but different conditions (true, false, false) (false, true, false) (false, false, true)
// - Block with condition false and old code is present
// - Block with condition false and no code is present

// Tests Outside Stacks
// - Generated files outside stacks are detected

func TestOutdatedDetection(t *testing.T) {
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
	t.Parallel()

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
			name: "generate_hcl is deleted when deleted",
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
				got, err = generate.DetectOutdated(s.Config())
				assert.NoError(t, err)
				assertEqualStringList(t, got, []string{})
			}
		})
	}
}
