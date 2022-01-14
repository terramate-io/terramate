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

package cli_test

import (
	"testing"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestCliMetadata(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		wd     string
		want   runExpected
	}

	for _, tc := range []testcase{
		{
			name: "no stacks",
		},
		{
			name: "one stack, wd = root",
			layout: []string{
				"s:stack",
			},
			want: runExpected{
				Stdout: `Available metadata:

stack "/stack":
	terramate.name="stack"
	terramate.path="/stack"
	terramate.description=""
`,
			},
		},
		{
			name: "multiple stacks, wd = root",
			layout: []string{
				"s:stack1",
				"s:stack2",
				"s:somedir/stack3",
				"s:somedir/stack4",
			},
			want: runExpected{
				Stdout: `Available metadata:

stack "/somedir/stack3":
	terramate.name="stack3"
	terramate.path="/somedir/stack3"
	terramate.description=""

stack "/somedir/stack4":
	terramate.name="stack4"
	terramate.path="/somedir/stack4"
	terramate.description=""

stack "/stack1":
	terramate.name="stack1"
	terramate.path="/stack1"
	terramate.description=""

stack "/stack2":
	terramate.name="stack2"
	terramate.path="/stack2"
	terramate.description=""
`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := newCLI(t, project.AbsPath(s.RootDir(), tc.wd))
			assertRunResult(t, cli.run("metadata"), tc.want)
		})
	}
}
