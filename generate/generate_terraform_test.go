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
	"github.com/mineiros-io/terramate/test/hclwrite"
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

	exportAsTerraform := func(label string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		b := hclwrite.BuildBlock("export_as_terraform", builders...)
		b.AddLabel(label)
		return b
	}
	provider := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("provider", builders...)
	}
	required_providers := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("required_providers", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}

	tcases := []testcase{
		{
			name: "no exported terraform",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name: "empty export_as_terraform block is ignored",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add:  exportAsTerraform("empty"),
				},
			},
		},
		{
			name: "export terraform for all stacks on parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: hcldoc(
						exportAsTerraform("backend",
							backend(
								labels("test"),
								expr("prefix", "global.backend_prefix"),
							),
						),
						exportAsTerraform("locals",
							locals(
								expr("stackpath", "terramate.path"),
								expr("local_a", "global.local_a"),
								expr("local_b", "global.local_b"),
								expr("local_c", "global.local_c"),
								expr("local_d", "try(global.local_d.field, null)"),
							),
						),
						exportAsTerraform("provider",
							provider(
								labels("name"),
								expr("data", "global.provider_data"),
							),
							terraform(
								required_providers(
									expr("name", `{
										source  = "integrations/name"
										version = global.provider_version
									}`),
								),
							),
							terraform(
								expr("required_version", "global.terraform_version"),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: globals(
						str("local_a", "stack-1-local"),
						boolean("local_b", true),
						number("local_c", 666),
						attr("local_d", `{ field = "local_d_field"}`),
						str("backend_prefix", "stack-1-backend"),
						str("provider_data", "stack-1-provider-data"),
						str("provider_version", "stack-1-provider-version"),
						str("terraform_version", "stack-1-terraform-version"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: globals(
						str("local_a", "stack-2-local"),
						boolean("local_b", false),
						number("local_c", 777),
						attr("local_d", `{ oopsie = "local_d_field"}`),
						str("backend_prefix", "stack-2-backend"),
						str("provider_data", "stack-2-provider-data"),
						str("provider_version", "stack-2-provider-version"),
						str("terraform_version", "stack-2-terraform-version"),
					),
				},
			},
			want: []want{
				{
					stack: "/stacks/stack-1",
					hcls: map[string]fmt.Stringer{
						"backend": backend(
							labels("test"),
							str("prefix", "stack-1-backend"),
						),

						"locals": locals(
							str("stackpath", "/stacks/stack-1"),
							str("local_a", "stack-1-local"),
							boolean("local_b", true),
							number("local_c", 666),
							str("local_d", "local_d_field"),
						),

						"provider": hcldoc(
							provider(
								labels("name"),
								str("data", "stack-1-provider-data"),
							),
							terraform(
								required_providers(
									expr("name", `{
										source  = "integrations/name"
										version = "stack-1-provider-version"
									}`),
								),
							),
							terraform(
								str("required_version", "stack-1-terraform-version"),
							),
						),
					},
				},
				{
					stack: "/stacks/stack-2",
					hcls: map[string]fmt.Stringer{
						"backend": backend(
							labels("test"),
							str("prefix", "stack-2-backend"),
						),
						"locals": locals(
							str("stackpath", "/stacks/stack-2"),
							str("local_a", "stack-2-local"),
							boolean("local_b", false),
							number("local_c", 777),
							attr("local_d", "null"),
						),
						"provider": hcldoc(
							provider(
								labels("name"),
								str("data", "stack-2-provider-data"),
							),
							terraform(
								required_providers(
									expr("name", `{
										source  = "integrations/name"
										version = "stack-2-provider-version"
									}`),
								),
							),
							terraform(
								str("required_version", "stack-2-terraform-version"),
							),
						),
					},
				},
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
			err = filepath.WalkDir(s.RootDir(), func(path string, d fs.DirEntry, err error) error {
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

			assert.NoError(t, err)
		})
	}
}
