// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gruntwork-io/go-commons/env"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestTerragruntParser(t *testing.T) {
	type want struct {
		err    error
		module *config.TerragruntConfig
	}

	type testcase struct {
		name    string
		baseDir string
		layout  []string
		want    want
	}

	pstr := func(s string) *string { return &s }

	for _, tc := range []testcase{
		{
			name: "empty terragrunt.hcl",
			want: want{
				err: os.ErrNotExist,
			},
		},
		{
			name: "simple terragrunt.hcl",
			layout: []string{
				`f:terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example"),
				).String(),
			},
			want: want{
				module: &config.TerragruntConfig{
					Terraform: &config.TerraformConfig{
						Source: pstr("github.com/hashicorp/terraform//example"),
					},
					IsPartial: true,
				},
			},
		},
		{
			name:    "simple terragrunt.hcl with dependencies block",
			baseDir: "target",
			layout: []string{
				`f:module1/terragrunt.hcl:` + Doc().String(),
				`f:target/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "github.com/hashicorp/terraform//example"),
					),
					Block("dependencies",
						Expr("paths", `["../module1"]`),
					)).String(),
			},
			want: want{
				module: &config.TerragruntConfig{
					IsPartial: true,
					Terraform: &config.TerraformConfig{
						Source: pstr("github.com/hashicorp/terraform//example"),
					},
					Dependencies: &config.ModuleDependencies{
						Paths: []string{"../module1"},
					},
				},
			},
		},
		{
			name:    "simple terragrunt.hcl with single dependency block",
			baseDir: "target",
			layout: []string{
				`f:module1/terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example"),
				).String(),
				`f:target/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "github.com/hashicorp/terraform//example"),
					),
					Block("dependency",
						Labels("module1"),
						Str("config_path", `../module1`),
					)).String(),
			},
			want: want{
				module: &config.TerragruntConfig{
					IsPartial: true,
					Terraform: &config.TerraformConfig{
						Source: pstr("github.com/hashicorp/terraform//example"),
					},
					TerragruntDependencies: []config.Dependency{
						{
							Name:       "module1",
							ConfigPath: "../module1",
						},
					},
					Dependencies: &config.ModuleDependencies{
						Paths: []string{"../module1"},
					},
				},
			},
		},
		{
			name:    "terragrunt.hcl with both dependency block and dependencies block",
			baseDir: "target",
			layout: []string{
				`f:module1/terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example1"),
				).String(),
				`f:module2/terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example2"),
				).String(),
				`f:target/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "github.com/hashicorp/terraform//example"),
					),
					Block("dependency",
						Labels("module1"),
						Str("config_path", `../module1`),
					),
					Block("dependencies",
						Expr("paths", `["../module2"]`),
					),
				).String(),
			},
			want: want{
				module: &config.TerragruntConfig{
					IsPartial: true,
					Terraform: &config.TerraformConfig{
						Source: pstr("github.com/hashicorp/terraform//example"),
					},
					TerragruntDependencies: []config.Dependency{
						{
							Name:       "module1",
							ConfigPath: "../module1",
						},
					},
					Dependencies: &config.ModuleDependencies{
						Paths: []string{"../module1", "../module2"},
					},
				},
			},
		},
		{
			name:    "terragrunt.hcl with dependency and dependencies sharing entries",
			baseDir: "target",
			layout: []string{
				`f:module1/terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example1"),
				).String(),
				`f:module2/terragrunt.hcl:` + Block("terraform",
					Str("source", "github.com/hashicorp/terraform//example2"),
				).String(),
				`f:target/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "github.com/hashicorp/terraform//example"),
					),
					Block("dependency",
						Labels("module1"),
						Str("config_path", `../module1`),
					),
					Block("dependency",
						Labels("module2"),
						Str("config_path", `../module2`),
					),
					Block("dependencies",
						Expr("paths", `["../module2", "../module1"]`),
					),
				).String(),
			},
			want: want{
				module: &config.TerragruntConfig{
					IsPartial: true,
					Terraform: &config.TerraformConfig{
						Source: pstr("github.com/hashicorp/terraform//example"),
					},
					TerragruntDependencies: []config.Dependency{
						{
							Name:       "module1",
							ConfigPath: "../module1",
						},
						{
							Name:       "module2",
							ConfigPath: "../module2",
						},
					},
					Dependencies: &config.ModuleDependencies{
						Paths: []string{"../module1", "../module2"},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, false)
			s.BuildTree(tc.layout)

			baseDir := s.RootDir()
			if tc.baseDir != "" {
				baseDir = filepath.Join(baseDir, tc.baseDir)
			}

			opts := newTerragruntOptions(baseDir)
			pctx := config.NewParsingContext(context.Background(), opts).WithDecodeList(
				// needed for tracking:
				//   - terraform.extra_arguments
				//   - terraform.required_vars_file
				//   - terraform.optional_var_files
				//   - etc
				config.TerraformBlock,

				// Needed for detecting modules.
				config.TerraformSource,

				// Need for parsing out the dependencies
				config.DependencyBlock,
				config.DependenciesBlock,
			)

			if tc.want.module != nil {
				for k, v := range tc.want.module.FieldsMetadata {
					for kk, vv := range v {
						if str, ok := vv.(string); kk == "found_in_file" && ok {
							tc.want.module.FieldsMetadata[k][kk] = filepath.Join(baseDir, str)
						}
					}
				}
			}

			got, err := config.PartialParseConfigFile(pctx, filepath.Join(baseDir, "terragrunt.hcl"), nil)
			if err != nil && tc.want.err == nil {
				t.Error(err)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("unexpected error: (-want +got)\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.module, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("unexpected module: (-want +got)\n%s", diff)
			}
		})
	}
}

func newTerragruntOptions(dir string) *options.TerragruntOptions {
	opts := options.NewTerragruntOptions()
	opts.RunTerragrunt = func(to *options.TerragruntOptions) error {

		return nil
	}
	opts.WorkingDir = dir
	opts.Writer = io.Discard
	opts.ErrWriter = io.Discard
	opts.IgnoreExternalDependencies = true
	opts.RunAllAutoApprove = false
	opts.AutoInit = false

	// very important, otherwise the functions could block with user prompts.
	opts.NonInteractive = true

	opts.Env = env.Parse(os.Environ())

	if opts.DisableLogColors {
		util.DisableLogColors()
	}

	opts.DownloadDir = util.JoinPath(opts.WorkingDir, util.TerragruntCacheDir)
	opts.TerragruntConfigPath = config.GetDefaultConfigPath(opts.WorkingDir)

	opts.OriginalTerragruntConfigPath = opts.TerragruntConfigPath
	opts.OriginalTerraformCommand = opts.TerraformCommand
	opts.OriginalIAMRoleOptions = opts.IAMRoleOptions

	return opts
}
