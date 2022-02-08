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
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestHCLGeneration(t *testing.T) {
	type (
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
		}
	)

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
			name: "no generated HCL",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name: "empty generate_hcl block is ignored",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add:  generateHCL(labels("empty")),
				},
			},
		},
		{
			name: "generate HCL for all stacks on parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: hcldoc(
						generateHCL(
							labels("backend.tf"),
							backend(
								labels("test"),
								expr("prefix", "global.backend_prefix"),
							),
						),
						generateHCL(
							labels("locals.tf"),
							locals(
								expr("stackpath", "terramate.path"),
								expr("local_a", "global.local_a"),
								expr("local_b", "global.local_b"),
								expr("local_c", "global.local_c"),
								expr("local_d", "try(global.local_d.field, null)"),
							),
						),
						generateHCL(
							labels("provider.tf"),
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
						"backend.tf": backend(
							labels("test"),
							str("prefix", "stack-1-backend"),
						),
						"locals.tf": locals(
							str("stackpath", "/stacks/stack-1"),
							str("local_a", "stack-1-local"),
							boolean("local_b", true),
							number("local_c", 666),
							str("local_d", "local_d_field"),
						),
						"provider.tf": hcldoc(
							provider(
								labels("name"),
								str("data", "stack-1-provider-data"),
							),
							terraform(
								required_providers(
									attr("name", `{
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
						"backend.tf": backend(
							labels("test"),
							str("prefix", "stack-2-backend"),
						),
						"locals.tf": locals(
							str("stackpath", "/stacks/stack-2"),
							str("local_a", "stack-2-local"),
							boolean("local_b", false),
							number("local_c", 777),
							attr("local_d", "null"),
						),
						"provider.tf": hcldoc(
							provider(
								labels("name"),
								str("data", "stack-2-provider-data"),
							),
							terraform(
								required_providers(
									attr("name", `{
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
		{
			name: "generate HCL with traversal of unknown namespaces",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: hcldoc(
						generateHCL(
							labels("traversal.tf"),
							block("traversal",
								expr("locals", "local.hi"),
								expr("some_anything", "something.should_work"),
								expr("multiple_traversal", "one.two.three.four.five"),
							),
						),
					),
				},
			},
			want: []want{
				{
					stack: "/stacks/stack-1",
					hcls: map[string]fmt.Stringer{
						"traversal.tf": hcldoc(
							block("traversal",
								expr("locals", "local.hi"),
								expr("multiple_traversal", "one.two.three.four.five"),
								expr("some_anything", "something.should_work"),
							),
						),
					},
				},
				{
					stack: "/stacks/stack-2",
					hcls: map[string]fmt.Stringer{
						"traversal.tf": hcldoc(
							block("traversal",
								expr("locals", "local.hi"),
								expr("multiple_traversal", "one.two.three.four.five"),
								expr("some_anything", "something.should_work"),
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
				cfg.Append(t, s.RootDir())
			}

			assertGeneratedHCLs := func(t *testing.T) {
				t.Helper()

				for _, wantDesc := range tcase.want {
					stackRelPath := wantDesc.stack[1:]
					stack := s.StackEntry(stackRelPath)

					for name, wantHCL := range wantDesc.hcls {
						want := wantHCL.String()
						got := stack.ReadGeneratedHCL(name)

						assertHCLEquals(t, got, want)
					}
				}
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			err := generate.Do(s.RootDir(), workingDir)
			assert.NoError(t, err)

			assertGeneratedHCLs(t)

			// piggyback on the tests to validate that regeneration doesnt
			// delete files or fail and has identical results.
			t.Run("regenerate", func(t *testing.T) {
				err := generate.Do(s.RootDir(), workingDir)
				assert.NoError(t, err)

				assertGeneratedHCLs(t)
			})

			// Check we don't have extraneous/unwanted files
			// We remove wanted/expected generated code
			// So we should have only basic terramate configs left
			// There is potential to extract this for other code generation tests.
			for _, wantDesc := range tcase.want {
				stackRelPath := wantDesc.stack[1:]
				stack := s.StackEntry(stackRelPath)
				for name := range wantDesc.hcls {
					stack.RemoveGeneratedHCL(name)
				}
			}
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

				t.Errorf("unwanted file at %q, got %q", path, d.Name())
				return nil
			})

			assert.NoError(t, err)
		})
	}
}

func TestWontOverwriteManuallyDefinedTerraform(t *testing.T) {
	const (
		genFilename  = "test.tf"
		manualTfCode = "some manual stuff, doesn't matter"
	)

	generateHCLConfig := generateHCL(
		labels(genFilename),
		terraform(
			str("required_version", "1.11"),
		),
	)

	s := sandbox.New(t)
	s.BuildTree([]string{
		fmt.Sprintf("f:%s:%s", config.Filename, generateHCLConfig.String()),
		"s:stack",
		fmt.Sprintf("f:stack/%s:%s", genFilename, manualTfCode),
	})

	err := generate.Do(s.RootDir(), s.RootDir())
	assert.IsError(t, err, generate.ErrManualCodeExists)

	stack := s.StackEntry("stack")
	actualTfCode := stack.ReadGeneratedHCL(genFilename)
	assert.EqualStrings(t, manualTfCode, actualTfCode, "tf code altered by generate")
}

func TestGenerateHCLOverwriting(t *testing.T) {
	const genFilename = "test.tf"

	firstConfig := generateHCL(
		labels(genFilename),
		terraform(
			str("required_version", "1.11"),
		),
	)
	firstWant := terraform(
		str("required_version", "1.11"),
	)

	s := sandbox.New(t)
	stack := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(firstConfig.String())

	s.Generate()

	got := stack.ReadGeneratedHCL(genFilename)
	assertHCLEquals(t, got, firstWant.String())

	secondConfig := generateHCL(
		labels(genFilename),
		terraform(
			str("required_version", "2.0"),
		),
	)
	secondWant := terraform(
		str("required_version", "2.0"),
	)

	rootConfig.Write(secondConfig.String())

	s.Generate()

	got = stack.ReadGeneratedHCL(genFilename)
	assertHCLEquals(t, got, secondWant.String())
}

func TestGeneratedHCLHeaders(t *testing.T) {
	const (
		rootFilename        = "root.tf"
		stackFilename       = "stack.tf"
		traceHeaderTemplate = "TERRAMATE: originated from generate_hcl block on %s"
	)

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")

	rootEntry.CreateConfig(
		generateHCL(
			labels(rootFilename),
			block("root",
				str("attr", "root"),
			),
		).String(),
	)

	stackEntry.CreateConfig(
		hcldoc(
			stack(),
			generateHCL(
				labels(stackFilename),
				block("stack",
					str("attr", "stack"),
				),
			),
		).String(),
	)

	s.Generate()

	stackGen := stackEntry.ReadGeneratedHCL(stackFilename)
	stackHeader := fmt.Sprintf(traceHeaderTemplate, filepath.Join("/stack", config.Filename))
	if !strings.Contains(stackGen, stackHeader) {
		t.Errorf("wanted header %q\n\ngenerated file:\n%s\n", stackHeader, stackGen)
	}

	rootGen := stackEntry.ReadGeneratedHCL(rootFilename)
	rootHeader := fmt.Sprintf(traceHeaderTemplate, "/"+config.Filename)
	if !strings.Contains(rootGen, rootHeader) {
		t.Errorf("wanted header %q\n\ngenerated file:\n%s\n", rootHeader, rootGen)
	}
}

func TestGenerateHCLCleanupOldFiles(t *testing.T) {
	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(
		hcldoc(
			generateHCL(
				labels("file1.tf"),
				block("block1",
					boolean("whatever", true),
				),
			),
			generateHCL(
				labels("file2.tf"),
				block("block2",
					boolean("whatever", true),
				),
			),
		).String(),
	)

	s.Generate()

	got := stackEntry.ListGenFiles()
	assertEqualStringList(t, got, []string{"file1.tf", "file2.tf"})

	rootConfig.Write(
		hcldoc(
			generateHCL(
				labels("file1.tf"),
				block("block1",
					boolean("whatever", true),
				),
			),
		).String(),
	)

	s.Generate()
	got = stackEntry.ListGenFiles()
	assertEqualStringList(t, got, []string{"file1.tf"})

	rootConfig.Write("")

	s.Generate()
	got = stackEntry.ListGenFiles()
	assertEqualStringList(t, got, []string{})
}
