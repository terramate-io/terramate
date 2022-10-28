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

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestGenerateDebug(t *testing.T) {
	type (
		file struct {
			path string
			body fmt.Stringer
		}
		testcase struct {
			name    string
			layout  []string
			wd      string
			configs []file
			want    runExpected
		}
	)
	t.Parallel()

	testcases := []testcase{
		{
			name: "empty project",
			want: runExpected{},
		},
		{
			name: "stacks with no codegen",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			want: runExpected{},
		},
		{
			name: "stacks with codegen on root",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", true),
							Str("content", "data"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Bool("condition", true),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
				{
					path: "stack-2/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Bool("condition", true),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
			},
			want: runExpected{
				Stdout: `Generated files for /stack-1:
	- file.hcl origin: /stack-1/config.tm:1,1-6,2
	- file.txt origin: /config.tm:5,1-8,2
Generated files for /stack-2:
	- file.hcl origin: /stack-2/config.tm:1,1-6,2
	- file.txt origin: /config.tm:5,1-8,2
`,
			},
		},
		{
			name: "stacks with codegen on stack",
			layout: []string{
				"s:stack-1",
				"s:stack-1/dir/child",
				"s:stack-2",
			},
			wd: "stack-1",
			configs: []file{
				{
					path: "config.tm",
					body: Doc(
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", false),
							Str("content", "data"),
						),
						GenerateFile(
							Labels("file.txt"),
							Bool("condition", true),
							Str("content", "data"),
						),
					),
				},
				{
					path: "stack-1/config.tm",
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Bool("condition", true),
							Content(
								Str("content", "data"),
							),
						),
					),
				},
			},
			want: runExpected{
				Stdout: `Generated files for /stack-1:
	- file.hcl origin: /stack-1/config.tm:1,1-6,2
	- file.txt origin: /config.tm:5,1-8,2
Generated files for /stack-1/dir/child:
	- file.hcl origin: /stack-1/config.tm:1,1-6,2
	- file.txt origin: /config.tm:5,1-8,2
`,
			},
		},
	}

	for _, tcase := range testcases {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			root := s.RootEntry()

			for _, config := range tc.configs {
				root.CreateFile(config.path, config.body.String())
			}

			ts := newCLI(t, filepath.Join(s.RootDir(), tc.wd))
			assertRunResult(t, ts.run("experimental", "generate", "debug"), tc.want)
		})
	}
}
