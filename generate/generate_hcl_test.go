// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/generate/genhcl"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
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
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"empty"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"empty"},
					},
				},
			},
		},
		{
			// This is a regression test for a severe bug on extension
			name: "multiple stacks extending imported globals on parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/module",
					filename: "config.tm",
					add: Globals(
						Labels("gclz_config", "terraform", "providers"),
						EvalExpr(t, "google", `{
							enabled = true
							source  = "hashicorp/google"
						}`),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Import(
						Str("source", "/module/config.tm"),
					),
				},
				{
					path:     "/stacks",
					filename: "config.tm",
					add: Doc(
						Globals(
							Labels("gclz_config", "terraform", "providers", "google"),
							Bool("enabled", false),
							Number("xxx", 666),
						),
						Globals(
							Bool("test", true),
						),

						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("gclz_config", "global.gclz_config"),
								Expr("test", "global.test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							EvalExpr(t, "gclz_config", `{
								terraform = {
								  providers = {
								    google = {
								      enabled = false
								      source  = "hashicorp/google"
								      xxx     = 666
								    }
								  }
								}
							}`),
							Bool("test", true),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							EvalExpr(t, "gclz_config", `{
								terraform = {
								  providers = {
								    google = {
								      enabled = false
								      source  = "hashicorp/google"
								      xxx     = 666
								    }
								  }
								}
							}`),
							Bool("test", true),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"file.hcl"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"file.hcl"},
					},
				},
			},
		},
		{
			name: "bug - reproducing iac-gcloud -- test tm_try",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path:     "/module",
					filename: "config.tm",
					add: Globals(
						Labels("gclz_config", "terraform", "providers"),
						Expr("google", `{
							enabled = true
							source  = "hashicorp/google"
							version = tm_try(global.gclz_terraform_google_provider_version, "4.33.0")
							config = {
								project = tm_try(global.gclz_terraform_google_provider_project, global.gclz_project_id)
							}
						}`),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Import(
							Str("source", "/module/config.tm"),
						),
					),
				},
				{
					path:     "/stacks",
					filename: "config.tm",
					add: Doc(
						Globals(
							Str("gclz_terraform_google_provider_version", "4.33.0"),
							Expr("gclz_project_id", `tm_try(global.lala, "test")`),
						),
						Globals(
							Labels("gclz_config", "terraform", "providers", "google"),
							Bool("enabled", false),
							Number("xxx", 666),
						),
						Globals(
							Bool("test", true),
						),

						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("gclz_config", "global.gclz_config"),
								Expr("test", "global.test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							EvalExpr(t, "gclz_config", `{
								terraform = {
								  providers = {
								    google = {
										config = {
											project = "test"
										}
										enabled = false
										source  = "hashicorp/google"
										version = "4.33.0"
								      	xxx     = 666
								    }
								  }
								}
							}`),
							Bool("test", true),
						),
					},
				},
				{
					dir: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							EvalExpr(t, "gclz_config", `{
								terraform = {
								  providers = {
								    google = {
										config = {
											project = "test"
										}
										enabled = false
										source  = "hashicorp/google"
										version = "4.33.0"
								      	xxx     = 666
								    }
								  }
								}
							}`),
							Bool("test", true),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"file.hcl"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
						Created: []string{"file.hcl"},
					},
				},
			},
		},
		{
			name: "bug - reproducing iac-gcloud -- test tm_try with more indirection",
			layout: []string{
				"s:stacks/stack-1",
			},
			configs: []hclconfig{
				{
					path:     "/module",
					filename: "config.tm",
					add: Globals(
						Labels("gclz_config", "terraform", "providers"),
						Expr("google", `{
							enabled = true
							source  = "hashicorp/google"
							version = tm_try(global.gclz_terraform_google_provider_version, "4.33.0")
							config = {
								project = tm_try(global.gclz_terraform_google_provider_project, global.gclz_project_id)
							}
						}`),
					),
				},
				{
					path:     "/",
					filename: "config.tm",
					add: Doc(
						Import(
							Str("source", "/module/config.tm"),
						),
					),
				},
				{
					path:     "/stacks",
					filename: "config.tm",
					add: Doc(
						Globals(
							Str("gclz_terraform_google_provider_version", "4.33.0"),
							Expr("gclz_project_id", `tm_try(global.lala, "test")`),
						),
						Globals(
							Labels("gclz_config", "terraform", "providers", "google"),
							Bool("enabled", false),
							Expr("xxx", "global.gclz_project_id"),
						),
						Globals(
							Bool("test", true),
						),

						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("gclz_config", "global.gclz_config"),
								Expr("test", "global.test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							EvalExpr(t, "gclz_config", `{
								terraform = {
								  providers = {
								    google = {
										config = {
											project = "test"
										}
										enabled = false
										source  = "hashicorp/google"
										version = "4.33.0"
								      	xxx     = "test"
								    }
								  }
								}
							}`),
							Bool("test", true),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"file.hcl"},
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
			name: "generate HCL with terramate.stacks.list",
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
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"stacks.hcl"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
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
						Dir:     project.NewPath("/"),
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
						Dir:     project.NewPath("/"),
						Created: []string{"root.hcl"},
					},
					{
						Dir:     project.NewPath("/stack-1"),
						Created: []string{"root.hcl"},
					},
					{
						Dir:     project.NewPath("/stack-2"),
						Created: []string{"root.hcl"},
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
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
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
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"backend.tf", "locals.tf", "provider.tf"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
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
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"traversal.tf"},
					},
					{
						Dir:     project.NewPath("/stacks/stack-2"),
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
							Dir: project.NewPath("/stacks/stack"),
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
							Dir: project.NewPath("/stacks/stack"),
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
						Dir:     project.NewPath("/stacks/stack"),
						Created: []string{"repeated"},
					},
				},
			},
		},
		{
			name: "generating embedded control characters as escape characters when plain strings",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Str("msg", "a\ttabbed\tstring"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test": Doc(
							Str("msg", "a\\ttabbed\\tstring"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "generating escaped control characters as escape characters when plain strings",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Expr("msg", `"a\ttabbed\tstring"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test": Doc(
							Str("msg", "a\\ttabbed\\tstring"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "generating rendered escape characters inside HEREDOC",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack",
					add: GenerateHCL(
						Labels("test"),
						Content(
							Expr("msg", `"a\n\ttabbed\tstring\n"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"test": Doc(
							Expr("msg", `<<-EOT
a
	tabbed	string
EOT
`),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack"),
						Created: []string{"test"},
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
							Dir: project.NewPath("/stacks/stack"),
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
							Dir: project.NewPath("/stacks/stack"),
						},
						Error: errors.E(generate.ErrConflictingConfig),
					},
				},
			},
		},
		{
			name: "generate HCL with dotfile",
			layout: []string{
				"s:/",
				"s:/stack-1",
				"s:/stack-2",
			},
			configs: []hclconfig{
				// inherited, fallthrough child stacks
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels(".tflint.hcl"),
							Content(
								Block("plugin",
									Labels("terraform"),
									Bool("enabled", true),
								),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						".tflint.hcl": Doc(
							Block("plugin",
								Labels("terraform"),
								Bool("enabled", true),
							),
						),
					},
				},
				{
					dir: "/stack-1",
					files: map[string]fmt.Stringer{
						".tflint.hcl": Doc(
							Block("plugin",
								Labels("terraform"),
								Bool("enabled", true),
							),
						),
					},
				},
				{
					dir: "/stack-2",
					files: map[string]fmt.Stringer{
						".tflint.hcl": Doc(
							Block("plugin",
								Labels("terraform"),
								Bool("enabled", true),
							),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/"),
						Created: []string{".tflint.hcl"},
					},
					{
						Dir:     project.NewPath("/stack-1"),
						Created: []string{".tflint.hcl"},
					},
					{
						Dir:     project.NewPath("/stack-2"),
						Created: []string{".tflint.hcl"},
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

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		fmt.Sprintf("f:%s:%s", config.DefaultFilename, generateHCLConfig.String()),
		"s:stack",
		fmt.Sprintf("f:stack/%s:%s", genFilename, manualTfCode),
	})

	report := generate.Do(s.Config(), project.NewPath("/modules"), nil)
	assert.EqualInts(t, 0, len(report.Successes), "want no success")
	assert.EqualInts(t, 1, len(report.Failures), "want single failure")
	assertReportHasError(t, report, errors.E(generate.ErrManualCodeExists))

	stack := s.StackEntry("stack")
	actualTfCode := stack.ReadFile(genFilename)
	assert.EqualStrings(t, manualTfCode, actualTfCode, "tf code altered by generate")
}

func TestGenerateHCLStackFilters(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "no matching pattern",
			layout: []string{
				"s:staecks/stack-1",
				"s:staecks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/staecks",
					add: GenerateHCL(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*"),
						),
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
			name: "one matching pattern",
			layout: []string{
				"s:stacks/stack-1",
				"s:staecks/stack-2",
				"s:stack/stack-3",
				"s:stackss/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*"),
						),
						Content(
							Backend(
								Labels("test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test": Backend(
							Labels("test"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "multiple matching patterns",
			layout: []string{
				"s:stacks/stack-1",
				"s:staecks/stack-2",
				"s:stack/stack-3",
				"s:staecks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("test"),
						StackFilter(
							ProjectPaths("stacks/*", "staecks/*"),
						),
						Content(
							Backend(
								Labels("test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"test": Backend(
							Labels("test"),
						),
					},
				},
				{
					dir: "/staecks/stack-2",
					files: map[string]fmt.Stringer{
						"test": Backend(
							Labels("test"),
						),
					},
				},
				{
					dir: "/staecks/stack-4",
					files: map[string]fmt.Stringer{
						"test": Backend(
							Labels("test"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/stack-1"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/staecks/stack-2"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/staecks/stack-4"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "escaped pattern",
			layout: []string{
				"s:st{a}ck\\s/stack-1",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("test"),
						StackFilter(
							ProjectPaths("st{a}ck\\\\s/*"),
						),
						Content(
							Backend(
								Labels("test"),
							),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/st{a}ck\\s/stack-1",
					files: map[string]fmt.Stringer{
						"test": Backend(
							Labels("test"),
						),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/st{a}ck\\s/stack-1"),
						Created: []string{"test"},
					},
				},
			},
		},
		{
			name: "AND multiple attributes",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("not_generated"),
						StackFilter(
							ProjectPaths("stack-1"),
							RepositoryPaths("stack-2"),
						),
						Content(),
					),
				},
			},
		},
		{
			name: "OR multiple blocks",
			layout: []string{
				"s:stack-1",
				"s:stack-2",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("generated"),
						StackFilter(
							ProjectPaths("stack-1"),
						),
						StackFilter(
							RepositoryPaths("stack-2"),
						),
						Content(Expr("foo", "bar")),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack-1",
					files: map[string]fmt.Stringer{
						"generated": Doc(Expr("foo", "bar")),
					},
				},
				{
					dir: "/stack-2",
					files: map[string]fmt.Stringer{
						"generated": Doc(Expr("foo", "bar")),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stack-1"),
						Created: []string{"generated"},
					},
					{
						Dir:     project.NewPath("/stack-2"),
						Created: []string{"generated"},
					},
				},
			},
		},
		{
			name: "glob patterns",
			layout: []string{
				"s:aws/stacks/dev",
				"s:aws/stacks/dev/substack",
				"s:aws/stacks/prod",
				"s:gcp/stacks/dev",
				"s:gcp/stacks/prod",
				"s:gcp/stacks/prod/substack",
				"s:dev",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("prod_match1"),
						StackFilter(
							ProjectPaths("prod"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("prod_match2"),
						StackFilter(
							ProjectPaths("**/prod"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("no_prod_match1"),
						StackFilter(
							ProjectPaths("*/prod"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("prod_substack_match1"),
						StackFilter(
							ProjectPaths("**/prod/*"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("prod_substack_match2"),
						StackFilter(
							ProjectPaths("**/prod/**"),
						),
						Content(),
					),
				},

				{
					path: "/",
					add: GenerateHCL(
						Labels("aws_prod_match1"),
						StackFilter(
							ProjectPaths("aws/**/prod"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("no_aws_substack_match1"),
						StackFilter(
							ProjectPaths("aws/*/substack"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("aws_substack_match1"),
						StackFilter(
							ProjectPaths("aws/**/substack"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("substack_match1"),
						StackFilter(
							ProjectPaths("substack"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("no_substack_match1"),
						StackFilter(
							ProjectPaths("/substack"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("root_dev_match1"),
						StackFilter(
							ProjectPaths("/dev"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("all_aws_match1"),
						StackFilter(
							ProjectPaths("aws/**"),
						),
						Content(),
					),
				},
				{
					path: "/",
					add: GenerateHCL(
						Labels("no_aws_match1"),
						StackFilter(
							ProjectPaths("aws/*"),
						),
						Content(),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/aws/stacks/dev",
					files: map[string]fmt.Stringer{
						"all_aws_match1": Doc(),
					},
				},
				{
					dir: "/aws/stacks/dev/substack",
					files: map[string]fmt.Stringer{
						"all_aws_match1":      Doc(),
						"aws_substack_match1": Doc(),
						"substack_match1":     Doc(),
					},
				},
				{
					dir: "/aws/stacks/prod",
					files: map[string]fmt.Stringer{
						"all_aws_match1":  Doc(),
						"aws_prod_match1": Doc(),
						"prod_match1":     Doc(),
						"prod_match2":     Doc(),
					},
				},
				{
					dir: "/dev",
					files: map[string]fmt.Stringer{
						"root_dev_match1": Doc(),
					},
				},
				{
					dir: "/gcp/stacks/prod",
					files: map[string]fmt.Stringer{
						"prod_match1": Doc(),
						"prod_match2": Doc(),
					},
				},
				{
					dir: "/gcp/stacks/prod/substack",
					files: map[string]fmt.Stringer{
						"prod_substack_match1": Doc(),
						"prod_substack_match2": Doc(),
						"substack_match1":      Doc(),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: project.NewPath("/aws/stacks/dev"),
						Created: []string{
							"all_aws_match1",
						},
					},
					{
						Dir: project.NewPath("/aws/stacks/dev/substack"),
						Created: []string{
							"all_aws_match1",
							"aws_substack_match1",
							"substack_match1",
						},
					},
					{
						Dir: project.NewPath("/aws/stacks/prod"),
						Created: []string{
							"all_aws_match1",
							"aws_prod_match1",
							"prod_match1",
							"prod_match2",
						},
					},
					{
						Dir: project.NewPath("/dev"),
						Created: []string{
							"root_dev_match1",
						},
					},
					{
						Dir: project.NewPath("/gcp/stacks/prod"),
						Created: []string{
							"prod_match1",
							"prod_match2",
						},
					},
					{
						Dir: project.NewPath("/gcp/stacks/prod/substack"),
						Created: []string{
							"prod_substack_match1",
							"prod_substack_match2",
							"substack_match1",
						},
					},
				},
			},
		},
	})
}

func TestGenerateHCLStackFiltersSC11177(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "bug-sc11177",
			layout: []string{
				"s:stacks/project-factory/factory-1/custom-roles/role-1",
				"s:stackss/project-factory/factory-2/custom-roles/role-2",
				"s:stacksss/project-factory/factory-3/custom-roles/role-3",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateHCL(
						Labels("test"),
						StackFilter(
							ProjectPaths(
								"**/project-factory/*/custom-roles/*",
								"**/projects/*/landing-zone/custom-roles/*",
							),
						),
						Content(),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/project-factory/factory-1/custom-roles/role-1",
					files: map[string]fmt.Stringer{
						"test": Doc(),
					},
				},
				{
					dir: "/stackss/project-factory/factory-2/custom-roles/role-2",
					files: map[string]fmt.Stringer{
						"test": Doc(),
					},
				},
				{
					dir: "/stacksss/project-factory/factory-3/custom-roles/role-3",
					files: map[string]fmt.Stringer{
						"test": Doc(),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     project.NewPath("/stacks/project-factory/factory-1/custom-roles/role-1"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/stackss/project-factory/factory-2/custom-roles/role-2"),
						Created: []string{"test"},
					},
					{
						Dir:     project.NewPath("/stacksss/project-factory/factory-3/custom-roles/role-3"),
						Created: []string{"test"},
					},
				},
			},
		},
	})
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

	s := sandbox.NoGit(t, true)
	stack := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(firstConfig.String())

	report := s.Generate()
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     project.NewPath("/stack"),
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
				Dir:     project.NewPath("/stack"),
				Changed: []string{genFilename},
			},
		},
	})

	got = stack.ReadFile(genFilename)
	test.AssertGenCodeEquals(t, got, secondWant.String())
	assertEqualReports(t, s.Generate(), generate.Report{})
}

func TestGenerateHCLCleanupFilesOnDirThatIsNotStack(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
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
				Dir:     project.NewPath("/stack"),
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     project.NewPath("/stack-2"),
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     project.NewPath("/stack/child"),
				Created: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     project.NewPath("/stack/child/grand"),
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
				Dir:     project.NewPath("/stack"),
				Deleted: []string{"file1.tf", "file2.tf"},
			},
			{
				Dir:     project.NewPath("/stack/child"),
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

	s := sandbox.NoGit(t, true)
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
				Dir:     project.NewPath("/stack"),
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
				Dir:     project.NewPath("/stack"),
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
				Dir:     project.NewPath("/stack"),
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
				Dir:     project.NewPath("/stack"),
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
				Dir:     project.NewPath("/stack"),
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

	s := sandbox.NoGit(t, true)
	rootEntry := s.RootEntry().CreateDir("root")
	stackEntry := s.CreateStack("root/stack")
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

	targEntry := s.RootEntry().CreateDir("target")
	linkPath := filepath.Join(stackEntry.Path(), "link")
	test.MkdirAll(t, targEntry.Path())
	assert.NoError(t, os.Symlink(targEntry.Path(), linkPath))

	// Creates a file with a generated header inside the symlinked directory.
	// It should never return in the report.
	test.WriteFile(t, targEntry.Path(), "test.tf", genhcl.DefaultHeader())

	root, err := config.LoadRoot(rootEntry.Path())
	assert.NoError(t, err)
	report := s.GenerateWith(root, project.NewPath("/modules"))
	assertEqualReports(t, report, generate.Report{
		Successes: []generate.Result{
			{
				Dir:     project.NewPath("/stack"),
				Created: []string{"file1.tf", "file2.tf"},
			},
		},
	})
}

func TestGenerateHCLCleanupOldFilesIgnoreDotDirs(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)

	// Creates a file with a generated header inside dot dirs.
	test.WriteFile(t, filepath.Join(s.RootDir(), ".terramate"), "test.tf", genhcl.DefaultHeader())
	test.WriteFile(t, filepath.Join(s.RootDir(), ".another"), "test.tf", genhcl.DefaultHeader())

	assertEqualReports(t, s.Generate(), generate.Report{})
}

func TestGenerateHCLCleanupOldFilesDONTIgnoreDotFiles(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)

	// Creates a file with a generated header inside dot dirs.
	test.WriteFile(t, filepath.Join(s.RootDir(), "somedir"), ".test.tf", genhcl.DefaultHeader())
	test.WriteFile(t, filepath.Join(s.RootDir(), "another"), ".test.tf", genhcl.DefaultHeader())

	assertEqualReports(t, s.Generate(), generate.Report{
		Successes: []generate.Result{
			{Dir: project.NewPath("/another"), Deleted: []string{".test.tf"}},
			{Dir: project.NewPath("/somedir"), Deleted: []string{".test.tf"}},
		},
	})
}

func TestGenerateHCLTerramateRootMetadata(t *testing.T) {
	t.Parallel()

	// We need to know the sandbox abspath to test terramate.root properly
	const generatedFile = "file.hcl"

	s := sandbox.NoGit(t, true)
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
				Dir:     project.NewPath("/stack"),
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
