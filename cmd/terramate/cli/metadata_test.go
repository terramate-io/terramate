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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestStackMetadata(t *testing.T) {

	type testcase struct {
		name    string
		layout  []string
		wantErr error
	}

	tcases := []testcase{
		{
			name:   "no stacks",
			layout: []string{},
		},
		{
			name:   "single stacks",
			layout: []string{"s:stack"},
		},
		{
			name: "two stacks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
		},
		{
			name: "three stacks and some non-stack dirs",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
				"s:stack-3",
				"d:non-stack",
				"d:non-stack-2",
			},
		},
		{
			name: "stacks nested",
			layout: []string{
				"s:envs/prod/stack-1",
				"s:envs/staging/stack-1",
			},
		},
		{
			name: "single invalid stack",
			layout: []string{
				fmt.Sprintf("f:invalid-stack/%s:data=notvalidhcl", terramate.ConfigFilename),
			},
			wantErr: hcl.ErrMalformedTerramateBlock,
		},
		{
			name: "valid stacks with invalid stack",
			layout: []string{
				"s:stack-valid-1",
				"s:stack-valid-2",
				fmt.Sprintf("f:invalid-stack/%s:data=notvalidhcl", terramate.ConfigFilename),
			},
			wantErr: hcl.ErrMalformedTerramateBlock,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			ts := newCLI(t, s.BaseDir())

			if tcase.wantErr != nil {
				assertRunResult(t, ts.run("metadata"), runResult{
					IgnoreStderr: true,
					Error:        tcase.wantErr,
				})
				return
			}

			want := "Available metadata:\n"

			for _, stack := range s.ListStacks() {
				projectPath := stackProjPath(s.BaseDir(), stack.Dir)
				want += fmt.Sprintf("\nstack %q:\n", projectPath)
				want += fmt.Sprintf("\tterraform.name=%q:\n", filepath.Base(projectPath))
				want += fmt.Sprintf("\tterraform.path=%q:\n", projectPath)
			}

			assertRunResult(t, ts.run("metadata"), runResult{Stdout: want})
		})
	}
}

func stackProjPath(basedir string, stackpath string) string {
	// As we refactor the stack type we may not need this anymore, since stacks
	// should known their absolute path relative to the project root.
	// Essentially this should go away soon :-)
	return strings.TrimPrefix(stackpath, basedir)
}
