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
	"testing"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// Most of the code generation behavior is tested
// through the generate pkg. Here we test the integration of code generation
// and vendoring. The way the integration is done has a good chance of
// changing so testing this in an e2e manner makes it less liable to
// break because of structural changes.

func TestGenerate(t *testing.T) {
	t.Parallel()

	type (
		file struct {
			path project.Path
			body fmt.Stringer
		}

		want struct {
			run   runExpected
			files []file
		}

		testcase struct {
			name   string
			layout []string
			files  []file
			want   want
		}
	)

	const noCodegenMsg = "Nothing to do, generated code is up to date\n"

	tcases := []testcase{
		{
			name: "no stacks",
			want: want{
				run: runExpected{
					Stdout: noCodegenMsg,
				},
			},
		},
		{
			name: "stacks with no codegen",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			want: want{
				run: runExpected{
					Stdout: noCodegenMsg,
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, file := range tcase.files {
				test.WriteFile(t,
					s.RootDir(),
					file.path.String(),
					file.body.String(),
				)
			}

			tmcli := newCLI(t, s.RootDir())
			res := tmcli.run("generate")
			assertRunResult(t, res, tcase.want.run)
		})
	}
}
