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
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

// Test
//
// - Overwriting Behavior (manual tf code already exists)

func TestTerraformGeneration(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		want struct {
			stack string
			hcls  map[string]fmt.Stringer
		}
		testcase struct {
			name       string
			layout     []string
			configs    []hclconfig
			workingDir string
			want       []want
			wantErr    error
		}
	)

	tcases := []testcase{
		{
			name: "no exported terraform",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.Filename, cfg.add.String())
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			err := generate.Do(s.RootDir(), workingDir)
			assert.IsError(t, err, tcase.wantErr)

			for _, wantDesc := range tcase.want {
				stackRelPath := wantDesc.stack[1:]
				stack := s.StackEntry(stackRelPath)

				for name, wantHCL := range wantDesc.hcls {
					want := wantHCL.String()
					got := string(stack.ReadGeneratedTerraform(name))

					assertHCLEquals(t, got, want)

					stack.RemoveGeneratedTerraform(name)
				}
			}

			// Check we don't have extraneous/unwanted files
			// Wanted/expected generated code was removed by this point
			// So we should have only basic terramate configs left
			// There is potential to extract this for other code generation tests.
			filepath.WalkDir(s.RootDir(), func(path string, d fs.DirEntry, err error) error {
				t.Helper()

				assert.NoError(t, err, "checking for unwanted generated files")
				if d.IsDir() {
					if d.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}

				// sandbox create README.md inside test dirs
				if d.Name() == config.Filename || d.Name() == "README.md" {
					return nil
				}

				t.Errorf("expected only basic terramate config at %q, got %q", path, d.Name())
				return nil
			})
		})
	}
}
