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
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestFilenameConflictsOnGeneration(t *testing.T) {
	// This test checks for how different code generation strategies
	// fail if they have conflicting filename configurations.
	// Like 2 different strategies that will write to the same filename.
	type testcase struct {
		name       string
		layout     []string
		configs    []hclconfig
		workingDir string
		want       error
	}

	gen := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate", builders...)
	}
	cfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("config", builders...)
	}

	tcases := []testcase{
		{
			name:   "export_as_locals conflicting with generate_hcl",
			layout: []string{"s:stacks/stack"},
			configs: []hclconfig{
				{
					path: "stacks",
					add: hcldoc(
						terramate(cfg(gen(
							str("locals_filename", "file.tf"),
						))),
						exportAsLocals(
							str("test", "val"),
						),
						generateHCL(
							labels("file.tf"),
							block("test"),
						),
					),
				},
			},
			want: generate.ErrConflictingConfig,
		},
		{
			name:   "empty export_as_locals conflicting with generate_hcl",
			layout: []string{"s:stacks/stack"},
			configs: []hclconfig{
				{
					path: "stacks",
					add: hcldoc(
						terramate(cfg(gen(
							str("locals_filename", "file.tf"),
						))),
						exportAsLocals(),
						generateHCL(
							labels("file.tf"),
							block("test"),
						),
					),
				},
			},
			want: generate.ErrConflictingConfig,
		},
		{
			name:   "export_as_locals conflicting with empty generate_hcl",
			layout: []string{"s:stacks/stack"},
			configs: []hclconfig{
				{
					path: "stacks",
					add: hcldoc(
						terramate(cfg(gen(
							str("locals_filename", "file.tf"),
						))),
						exportAsLocals(
							str("test", "hi"),
						),
						generateHCL(
							labels("file.tf"),
						),
					),
				},
			},
			want: generate.ErrConflictingConfig,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				cfg.append(t, s.RootDir())
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			err := generate.Do(s.RootDir(), workingDir)
			assert.IsError(t, err, tcase.want)
		})
	}
}
