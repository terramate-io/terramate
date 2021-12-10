// Copyright 2021 Mineiros GmbH
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

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// TODO(katcipis)
//
// - Empty backend block
// - backend block with empty block inside
// - backend block with block inside with random attrs
// - backend block at project root
// - backend block on different envs subdirs

func TestBackendConfigGeneration(t *testing.T) {
	type stackcode struct {
		relpath string
		code    string
	}
	type want struct {
		res    runResult
		stacks []stackcode
	}

	type testcase struct {
		name   string
		layout []string
		want   want
	}

	tests := []testcase{
		{
			name:   "single stack with config on it",
			layout: []string{"s:stack"},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "sometype" {
    attr = "value"
  }
}
`,
					},
				},
				res: runResult{IgnoreStdout: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(test.layout)

			stackconfig := `terramate {
  required_version = "~> 0.0.0"
  backend "sometype" {
    attr = "value"
  }
}`
			stack := s.StackEntry("stack")
			stack.CreateConfig(stackconfig)
			ts := newCLI(t, s.BaseDir())

			assertRunResult(t, ts.run("generate"), test.want.res)

			for _, want := range test.want.stacks {
				stack := s.StackEntry(want.relpath)
				got := string(stack.ReadGeneratedTf())
				wantcode := terramate.GeneratedCodeHeader + "\n\n" + want.code

				if diff := cmp.Diff(wantcode, got); diff != "" {
					t.Error("generated code doesn't match expectation")
					t.Errorf("want:\n%q", wantcode)
					t.Errorf("got:\n%q", got)
					t.Fatalf("diff:\n%s", diff)
				}
			}
		})
	}

}
