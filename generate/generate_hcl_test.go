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
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestGenerateHCL(t *testing.T) {
	t.Parallel()

	provider := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("provider", builders...)
	}
	requiredProviders := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("required_providers", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return EvalExpr(t, name, expr)
	}

	testCodeGeneration(t, []testcase{
		{
			name: "no generated HCL",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name: "empty generate_hcl block generates empty file",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("empty"),
						Content(),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"empty": Doc(),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"empty": Doc(),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"empty"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"empty"},
					},
				},
			},
		},
		{
			name: "generate_hcl with false condition generates nothing",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("test"),
						Bool("condition", false),
						Content(
							Backend(
								Labels("test"),
							),
						),
					),
				},
			},
		},
		{
			name: "generate HCL with terramate.stacks.list with workdir on root",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateHCL(
							Labels("stacks.hcl"),
							Content(
								Expr("stacks", "terramate.stacks.list"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"stacks.hcl": Doc(
							attr("stacks", `["/stacks/stack-1", "/stacks/stack-2"]`),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"stacks.hcl": Doc(
							attr("stacks", `["/stacks/stack-1", "/stacks/stack-2"]`),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"stacks.hcl"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"stacks.hcl"},
					},
				},
			},
		},
		{
			name: "generate HCL with stack on root",
			layout: []string{
				"s:/",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("root.hcl"),
							Content(
								Expr("stacks", "terramate.stacks.list"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"root.hcl": Doc(
							attr("stacks", `["/"]`),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/",
						Created: []string{"root.hcl"},
					},
				},
			},
		},
		{
			name: "generate HCL with stack on root and substacks",
			layout: []string{
				"s:/",
				"s:/stack-1",
				"s:/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("root.hcl"),
							Content(
								Expr("stacks", "terramate.stacks.list"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"root.hcl": Doc(
							attr("stacks", `["/", "/stack-1", "/stack-2"]`),
						),
					},
				},
				{
					dir: "/stack-1",
					files: map[string]fmt.Stringer{
						"root.hcl": Doc(
							attr("stacks", `["/", "/stack-1", "/stack-2"]`),
						),
					},
				},
				{
					dir: "/stack-2",
					files: map[string]fmt.Stringer{
						"root.hcl": Doc(
							attr("stacks", `["/", "/stack-1", "/stack-2"]`),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/",
						Created: []string{"root.hcl"},
					},
					{
						Dir:     "/stack-1",
						Created: []string{"root.hcl"},
					},
					{
						Dir:     "/stack-2",
						Created: []string{"root.hcl"},
					},
				},
			},
		},
		{
			name: "generate HCL with terramate.stacks.list with workdir on stack",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-1",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateHCL(
							Labels("stacks.hcl"),
							Content(
								Expr("stacks", "terramate.stacks.list"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"stacks.hcl": Doc(
							attr("stacks", `["/stacks/stack-1", "/stacks/stack-2"]`),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"stacks.hcl"},
					},
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
					add: Doc(
						GenerateHCL(
							Labels("backend.tf"),
							Content(
								Backend(
									Labels("test"),
									Expr("prefix", "global.backend_prefix"),
								),
							),
						),
						GenerateHCL(
							Labels("locals.tf"),
							Content(
								Locals(
									Expr("stackpath", "terramate.path"),
									Expr("local_a", "global.local_a"),
									Expr("local_b", "global.local_b"),
									Expr("local_c", "global.local_c"),
									Expr("local_d", "tm_try(global.local_d.field, null)"),
								),
							),
						),
						GenerateHCL(
							Labels("provider.tf"),
							Content(
								provider(
									Labels("name"),
									Expr("data", "global.provider_data"),
								),
								Terraform(
									requiredProviders(
										Expr("name", `{
										source  = "integrations/name"
										version = global.provider_version
									}`),
									),
								),
								Terraform(
									Expr("required_version", "global.terraform_version"),
								),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Globals(
						Str("local_a", "stack-1-local"),
						Bool("local_b", true),
						Number("local_c", 666),
						attr("local_d", `{ field = "local_d_field"}`),
						Str("backend_prefix", "stack-1-backend"),
						Str("provider_data", "stack-1-provider-data"),
						Str("provider_version", "stack-1-provider-version"),
						Str("terraform_version", "stack-1-terraform-version"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Globals(
						Str("local_a", "stack-2-local"),
						Bool("local_b", false),
						Number("local_c", 777),
						attr("local_d", `{ oopsie = "local_d_field"}`),
						Str("backend_prefix", "stack-2-backend"),
						Str("provider_data", "stack-2-provider-data"),
						Str("provider_version", "stack-2-provider-version"),
						Str("terraform_version", "stack-2-terraform-version"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"backend.tf": Backend(
							Labels("test"),
							Str("prefix", "stack-1-backend"),
						),
						"locals.tf": Locals(
							Str("local_a", "stack-1-local"),
							Bool("local_b", true),
							Number("local_c", 666),
							Str("local_d", "local_d_field"),
							Str("stackpath", "/stacks/stack-1"),
						),
						"provider.tf": Doc(
							provider(
								Labels("name"),
								Str("data", "stack-1-provider-data"),
							),
							Terraform(
								requiredProviders(
									attr("name", `{
										source  = "integrations/name"
										version = "stack-1-provider-version"
									}`),
								),
							),
							Terraform(
								Str("required_version", "stack-1-terraform-version"),
							),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"backend.tf": Backend(
							Labels("test"),
							Str("prefix", "stack-2-backend"),
						),
						"locals.tf": Locals(
							Str("local_a", "stack-2-local"),
							Bool("local_b", false),
							Number("local_c", 777),
							attr("local_d", "null"),
							Str("stackpath", "/stacks/stack-2"),
						),
						"provider.tf": Doc(
							provider(
								Labels("name"),
								Str("data", "stack-2-provider-data"),
							),
							Terraform(
								requiredProviders(
									attr("name", `{
										source  = "integrations/name"
										version = "stack-2-provider-version"
									}`),
								),
							),
							Terraform(
								Str("required_version", "stack-2-terraform-version"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
					},
				},
			},
		},
		{
			name: "generate HCL for all stacks importing common",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/common",
					add: Doc(
						GenerateHCL(
							Labels("backend.tf"),
							Content(
								Backend(
									Labels("test"),
									Expr("prefix", "global.backend_prefix"),
								),
							),
						),
						GenerateHCL(
							Labels("locals.tf"),
							Content(
								Locals(
									Expr("stackpath", "terramate.path"),
									Expr("local_a", "global.local_a"),
									Expr("local_b", "global.local_b"),
									Expr("local_c", "global.local_c"),
									Expr("local_d", "tm_try(global.local_d.field, null)"),
								),
							),
						),
						GenerateHCL(
							Labels("provider.tf"),
							Content(
								provider(
									Labels("name"),
									Expr("data", "global.provider_data"),
								),
								Terraform(
									requiredProviders(
										Expr("name", `{
										source  = "integrations/name"
										version = global.provider_version
									}`),
									),
								),
								Terraform(
									Expr("required_version", "global.terraform_version"),
								),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: Doc(
						Import(
							Str("source", fmt.Sprintf("/common/%s", config.DefaultFilename)),
						),
						Globals(
							Str("local_a", "stack-1-local"),
							Bool("local_b", true),
							Number("local_c", 666),
							attr("local_d", `{ field = "local_d_field"}`),
							Str("backend_prefix", "stack-1-backend"),
							Str("provider_data", "stack-1-provider-data"),
							Str("provider_version", "stack-1-provider-version"),
							Str("terraform_version", "stack-1-terraform-version"),
						),
					),
				},
				{
					path: "/stacks/stack-2",
					add: Doc(
						Import(
							Str("source", fmt.Sprintf("/common/%s", config.DefaultFilename)),
						),
						Globals(
							Str("local_a", "stack-2-local"),
							Bool("local_b", false),
							Number("local_c", 777),
							attr("local_d", `{ oopsie = "local_d_field"}`),
							Str("backend_prefix", "stack-2-backend"),
							Str("provider_data", "stack-2-provider-data"),
							Str("provider_version", "stack-2-provider-version"),
							Str("terraform_version", "stack-2-terraform-version"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"backend.tf": Backend(
							Labels("test"),
							Str("prefix", "stack-1-backend"),
						),
						"locals.tf": Locals(
							Str("local_a", "stack-1-local"),
							Bool("local_b", true),
							Number("local_c", 666),
							Str("local_d", "local_d_field"),
							Str("stackpath", "/stacks/stack-1"),
						),
						"provider.tf": Doc(
							provider(
								Labels("name"),
								Str("data", "stack-1-provider-data"),
							),
							Terraform(
								requiredProviders(
									attr("name", `{
										source  = "integrations/name"
										version = "stack-1-provider-version"
									}`),
								),
							),
							Terraform(
								Str("required_version", "stack-1-terraform-version"),
							),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"backend.tf": Backend(
							Labels("test"),
							Str("prefix", "stack-2-backend"),
						),
						"locals.tf": Locals(
							Str("local_a", "stack-2-local"),
							Bool("local_b", false),
							Number("local_c", 777),
							attr("local_d", "null"),
							Str("stackpath", "/stacks/stack-2"),
						),
						"provider.tf": Doc(
							provider(
								Labels("name"),
								Str("data", "stack-2-provider-data"),
							),
							Terraform(
								requiredProviders(
									attr("name", `{
										source  = "integrations/name"
										version = "stack-2-provider-version"
									}`),
								),
							),
							Terraform(
								Str("required_version", "stack-2-terraform-version"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
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
					add: Doc(
						GenerateHCL(
							Labels("traversal.tf"),
							Content(
								Block("traversal",
									Expr("locals", "local.hi"),
									Expr("some_anything", "something.should_work"),
									Expr("multiple_traversal", "one.two.three.four.five"),
								),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"traversal.tf": Doc(
							Block("traversal",
								Expr("locals", "local.hi"),
								Expr("multiple_traversal", "one.two.three.four.five"),
								Expr("some_anything", "something.should_work"),
							),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"traversal.tf": Doc(
							Block("traversal",
								Expr("locals", "local.hi"),
								Expr("multiple_traversal", "one.two.three.four.five"),
								Expr("some_anything", "something.should_work"),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack-1",
						Created: []string{"traversal.tf"},
					},
					{
						Dir:     "/stacks/stack-2",
						Created: []string{"traversal.tf"},
					},
				},
			},
		},
		{
			name: "stack with block with same label as parent",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "stack data"),
							),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "stack imports config with block with same label as parent",
			layout: []string{
				"s:stacks/stack",
				"d:other",
			},
			configs: []hclconfig{
				{
					path: "/other",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "imported data"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "stacks data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: Import(
						Str("source", fmt.Sprintf("/other/%s", config.DefaultFilename)),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "stack with block with same label as parent but different condition",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
						Bool("condition", false),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "stack data"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"repeated": Block("block",
							Str("data", "stack data"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stacks/stack",
						Created: []string{"repeated"},
					},
				},
			},
		},
		{
			name: "stack with block with same label as parent but multiple true conditions",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
						Bool("condition", true),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
						Bool("condition", false),
					),
				},
				{
					path: "/stacks/stack",
					add: GenerateHCL(
						Labels("repeated"),
						Bool("condition", true),
						Content(
							Block("block",
								Str("data", "stack data"),
							),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "stack parents with block with same label is an error",
			layout: []string{
				"s:stacks/stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "root data"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: GenerateHCL(
						Labels("repeated"),
						Content(
							Block("block",
								Str("data", "parent data"),
							),
						),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stacks/stack",
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
	})
}

func TestWontOverwriteManuallyDefinedTerraform(t *testing.T) {
	t.Parallel()

	const (
		genFilename  = "test.tf"
		manualTfCode = "some manual stuff, doesn't matter"
	)

	generateHCLConfig := GenerateHCL(
		Labels(genFilename),
		Content(
			Terraform(
				Str("required_version", "1.11"),
			),
		),
	)

	s := sandbox.New(t)
	s.BuildTree([]string{
		fmt.Sprintf("f:%s:%s", config.DefaultFilename, generateHCLConfig.String()),
		"s:stack",
		fmt.Sprintf("f:stack/%s:%s", genFilename, manualTfCode),
	})

	report := generate.Do(s.Config(), s.RootDir())
	assert.EqualInts(t, 0, len(report.Successes), "want no success")
	assert.EqualInts(t, 1, len(report.Failures), "want single failure")
	assertReportHasError(t, report, errors.E(generate.ErrManualCodeExists))

	stack := s.StackEntry("stack")
	actualTfCode := stack.ReadFile(genFilename)
	assert.EqualStrings(t, manualTfCode, actualTfCode, "tf code altered by generate")
}

func TestGenerateHCLOverwriting(t *testing.T) {
	t.Parallel()

	const genFilename = "test.tf"

	firstConfig := GenerateHCL(
		Labels(genFilename),
		Content(
			Terraform(
				Str("required_version", "1.11"),
			),
		),
	)
	firstWant := Terraform(
		Str("required_version", "1.11"),
	)

	s := sandbox.New(t)
	stack := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(firstConfig.String())

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{genFilename},
			},
		},
	})

	got := stack.ReadFile(genFilename)
	test.AssertGenCodeEquals(t, got, firstWant.String())

	secondConfig := GenerateHCL(
		Labels(genFilename),
		Content(
			Terraform(
				Str("required_version", "2.0"),
			),
		),
	)
	secondWant := Terraform(
		Str("required_version", "2.0"),
	)

	rootConfig.Write(secondConfig.String())

	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Changed: []string{genFilename},
			},
		},
	})

	got = stack.ReadFile(genFilename)
	test.AssertGenCodeEquals(t, got, secondWant.String())
	assertEqualReports(t, s.Generate(), generate.Report{})
}

func TestGeneratedHCLHeaders(t *testing.T) {
	t.Parallel()

	const (
		rootFilename        = "root.tf"
		stackFilename       = "stack.tf"
		traceHeaderTemplate = "TERRAMATE: originated from generate_hcl block on %s"
	)

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")

	rootEntry.CreateConfig(
		GenerateHCL(
			Labels(rootFilename),
			Content(
				Block("root",
					Str("attr", "root"),
				),
			),
		).String(),
	)

	stackEntry.CreateConfig(
		GenerateHCL(
			Labels(stackFilename),
			Content(
				Block("stack",
					Str("attr", "stack"),
				),
			),
		).String(),
	)

	s.Generate()

	stackGen := stackEntry.ReadFile(stackFilename)
	stackHeader := fmt.Sprintf(traceHeaderTemplate, path.Join("/stack", config.DefaultFilename))
	if !strings.Contains(stackGen, stackHeader) {
		t.Errorf("wanted header %q\n\ngenerated file:\n%s\n", stackHeader, stackGen)
	}

	rootGen := stackEntry.ReadFile(rootFilename)
	rootHeader := fmt.Sprintf(traceHeaderTemplate, "/"+config.DefaultFilename)
	if !strings.Contains(rootGen, rootHeader) {
		t.Errorf("wanted header %q\n\ngenerated file:\n%s\n", rootHeader, rootGen)
	}
}

func TestGenerateHCLCleanupFilesOnDirThatIsNotStack(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	childStack := s.CreateStack("stack/child")
	grandChildStack := s.CreateStack("stack/child/grand")
	stack2Entry := s.CreateStack("stack-2")

	rootEntry := s.DirEntry(".")
	rootEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("file1.tf"),
				Content(
					Block("block1",
						Bool("whatever", true),
					),
				),
			),
			GenerateHCL(
				Labels("file2.tf"),
				Content(
					Block("block2",
						Bool("whatever", true),
					),
				),
			),
		).String(),
	)

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     "/stack-2",
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     "/stack/child",
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     "/stack/child/grand",
				Created: []string{"file1.tf", "file2.tf"},
			},
		},
	})

	stackEntry.DeleteStackConfig()
	grandChildStack.DeleteStackConfig()

	s.ReloadConfig()
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Deleted: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     "/stack/child",
				Deleted: []string{"grand/file1.tf", "grand/file2.tf"},
			},
		},
	})

	assertEqualStringList(t, stackEntry.ListGenFiles(s.Config()), []string{})
	assertEqualStringList(t, grandChildStack.ListGenFiles(s.Config()), []string{})

	assertEqualStringList(t, childStack.ListGenFiles(s.Config()),
		[]string{"file1.tf", "file2.tf"})
	assertEqualStringList(t, stack2Entry.ListGenFiles(s.Config()),
		[]string{"file1.tf", "file2.tf"})
}

func TestGenerateHCLCleanupOldFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(
		Doc(
			GenerateHCL(
				Labels("file1.tf"),
				Content(
					Block("block1",
						Bool("whatever", true),
					),
				),
			),
			GenerateHCL(
				Labels("file2.tf"),
				Content(
					Block("block2",
						Bool("whatever", true),
					),
				),
			),
		).String(),
	)

	s.ReloadConfig()
	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{"file1.tf", "file2.tf"},
			},
		},
	})

	got := stackEntry.ListGenFiles(s.Config())
	assertEqualStringList(t, got, []string{"file1.tf", "file2.tf"})

	// Lets change one of the files, but delete the other
	rootConfig.Write(
		Doc(
			GenerateHCL(
				Labels("file1.tf"),
				Content(
					Block("changed",
						Bool("newstuff", true),
					),
				),
			),
		).String(),
	)

	s.ReloadConfig()
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Changed: []string{"file1.tf"},
				Deleted: []string{"file2.tf"},
			},
		},
	})

	got = stackEntry.ListGenFiles(s.Config())
	assertEqualStringList(t, got, []string{"file1.tf"})

	// condition = false gets deleted
	rootConfig.Write(
		Doc(
			GenerateHCL(
				Labels("file1.tf"),
				Bool("condition", false),
				Content(),
			),
		).String(),
	)

	s.ReloadConfig()
	report = s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Deleted: []string{"file1.tf"},
			},
		},
	})

	// Block with condition = false will be ignored
	rootConfig.Write(
		Doc(
			GenerateHCL(
				Labels("file1.tf"),
				Bool("condition", false),
				Content(
					Block("test",
						Bool("test", true),
					),
				),
			),
			GenerateHCL(
				Labels("file2.tf"),
				Bool("condition", true),
				Content(
					Block("test",
						Bool("test", true),
					),
				),
			),
		).String(),
	)

	s.ReloadConfig()
	assertEqualReports(t, s.Generate(), generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{"file2.tf"},
			},
		},
	})
	got = stackEntry.ListGenFiles(s.Config())
	assertEqualStringList(t, got, []string{"file2.tf"})

	// Block changed to condition = false will be deleted
	rootConfig.Write(
		Doc(
			GenerateHCL(
				Labels("file2.tf"),
				Bool("condition", false),
				Content(
					Block("test",
						Bool("test", true),
					),
				),
			),
		).String(),
	)

	s.ReloadConfig()
	assertEqualReports(t, s.Generate(), generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Deleted: []string{"file2.tf"},
			},
		},
	})
	got = stackEntry.ListGenFiles(s.Config())
	assertEqualStringList(t, got, []string{})
}

func TestGenerateHCLCleanupOldFilesIgnoreSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipped on windows because it requires privileges")
	}
	t.Parallel()

	s := sandbox.NoGit(t)
	rootEntry := s.RootEntry().CreateDir("root")
	stackEntry := s.CreateStack("root/stack")
	rootEntry.CreateConfig(
		Doc(
			Terramate(
				Config(),
			),
			GenerateHCL(
				Labels("file1.tf"),
				Content(
					Block("block1",
						Bool("whatever", true),
					),
				),
			),
			GenerateHCL(
				Labels("file2.tf"),
				Content(
					Block("block2",
						Bool("whatever", true),
					),
				),
			),
		).String(),
	)

	targEntry := s.RootEntry().CreateDir("target")
	linkPath := filepath.Join(stackEntry.Path(), "link")
	test.MkdirAll(t, targEntry.Path())
	assert.NoError(t, os.Symlink(targEntry.Path(), linkPath))

	// Creates a file with a generated header inside the symlinked directory.
	// It should never return in the report.
	test.WriteFile(t, targEntry.Path(), "test.tf", genhcl.Header)

	cfg, err := config.LoadTree(rootEntry.Path(), rootEntry.Path())
	assert.NoError(t, err)
	report := s.GenerateAt(cfg, rootEntry.Path())
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{"file1.tf", "file2.tf"},
			},
		},
	})
}

func TestGenerateHCLCleanupOldFilesIgnoreDotDirs(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t)

	// Creates a file with a generated header inside dot dirs.
	test.WriteFile(t, filepath.Join(s.RootDir(), ".terramate"), "test.tf", genhcl.Header)
	test.WriteFile(t, filepath.Join(s.RootDir(), ".another"), "test.tf", genhcl.Header)

	assertEqualReports(t, s.Generate(), generate.Report{})
}

func TestGenerateHCLTerramateRootMetadata(t *testing.T) {
	t.Parallel()

	// We need to know the sandbox abspath to test terramate.root properly
	const generatedFile = "file.hcl"

	s := sandbox.New(t)
	stackEntry := s.CreateStack("stack")
	s.RootEntry().CreateConfig(
		Doc(
			GenerateHCL(
				Labels(generatedFile),
				Content(
					Expr("terramate_root_path_abs", "terramate.root.path.fs.absolute"),
					Expr("terramate_root_path_basename", "terramate.root.path.fs.basename"),
				),
			),
		).String(),
	)

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     "/stack",
				Created: []string{generatedFile},
			},
		},
	})

	want := Doc(
		Str("terramate_root_path_abs", escapeBackslash(s.RootDir())),
		Str("terramate_root_path_basename", filepath.Base(s.RootDir())),
	).String()
	got := stackEntry.ReadFile(generatedFile)

	test.AssertGenCodeEquals(t, got, want)
}

func escapeBackslash(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}
