// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/project"
	errtest "github.com/terramate-io/terramate/test/errors"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

func TestTerragruntScanModules(t *testing.T) {
	t.Parallel()
	type want struct {
		err     error
		modules tg.Modules
	}
	type testcase struct {
		name       string
		layout     []string
		basedir    project.Path
		ignoreDeps bool
		want       want
	}

	for _, tc := range []testcase{
		{
			name: "no terragrunt files",
		},
		{
			name: "empty config file",
			layout: []string{
				`f:terragrunt.hcl:`,
			},
		},
		{
			name: "config without terraform.source",
			layout: []string{
				`f:terragrunt.hcl:` + Block("terraform").String(),
			},
		},
		{
			name: "invalid configuration",
			layout: []string{
				`f:terragrunt.hcl:` + Block("terraform",
					Str("unknown", "unknown field"),
				).String(),
			},
			want: want{
				err: errors.E(tg.ErrParsing),
			},
		},
		{
			name: "single module at root",
			layout: []string{
				`f:terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "module with hooks using if condition",
			layout: []string{
				`f:terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
						Block("before_hook", Labels("validate"),
							Expr("commands", `["plan", "apply"]`),
							Expr("execute", `["terraform", "validate"]`),
							Expr("run_on_error", "false"),
						),
						Block("before_hook", Labels("conditional"),
							Expr("commands", `["apply"]`),
							Expr("execute", `["echo", "Running apply"]`),
							Expr("if", `get_env("ENABLE_HOOK", "false") == "true"`),
						),
					),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "leaf single module",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "module inside hidden dir is ignored",
			layout: []string{
				`f:some/.dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
		},
		{
			name: "module inside dir with parent .tmskip is ignored",
			layout: []string{
				`f:some/.tmskip:`,
				`f:some/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
		},
		{
			name: "module inside dir with .tmskip is same level is ignored",
			layout: []string{
				`f:some/.tmskip:`,
				`f:some/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
		},
		{
			name: "multiple leaf modules",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:some/other/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
					},
					{
						Path:       project.NewPath("/some/other/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "module with dependency + root file",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("include",
						Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),
					Block("dependency",
						Labels("other2"), // other2 declared before other: result must be sorted.
						Str("config_path", "../other2/dir"),
					),
					Block("dependency",
						Labels("other"),
						Str("config_path", "../other/dir"),
					),
				).String(),
				`f:some/other/dir/terragrunt.hcl:` + Doc(
					Bool("skip", true),
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					)).String(),
				`f:some/other2/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:terragrunt.hcl:` + Doc(
					Block("terraform"),
				).String(),
				`f:common.tfvars:a = "1"`,
				`f:regional.tfvars:b = "2"`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
							project.NewPath("/terragrunt.hcl"),
						},
						After: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
						DependencyBlocks: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
					},
					{
						Path:       project.NewPath("/some/other/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other/dir/terragrunt.hcl"),
					},
					{
						Path:       project.NewPath("/some/other2/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other2/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "module with ordering dependencies",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("include",
						Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),
					Block("dependencies",
						Expr("paths", `["../other2/dir", "../other/dir"]`),
					),
				).String(),
				`f:some/other/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:some/other2/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:terragrunt.hcl:` + Doc(
					Block("terraform"),
				).String(),
				`f:common.tfvars:a = "1"`,
				`f:regional.tfvars:b = "2"`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/terragrunt.hcl"),
						},
						After: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
					},
					{
						Path:       project.NewPath("/some/other/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other/dir/terragrunt.hcl"),
					},
					{
						Path:       project.NewPath("/some/other2/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other2/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "module with ordering dependencies",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("include",
						Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),
					Block("dependencies",
						Expr("paths", `["../other2/dir", "../other/dir"]`),
					),
				).String(),
				`f:some/other/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:some/other2/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:terragrunt.hcl:` + Doc(
					Block("terraform"),
				).String(),
				`f:common.tfvars:a = "1"`,
				`f:regional.tfvars:b = "2"`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/terragrunt.hcl"),
						},
						After: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
					},
					{
						Path:       project.NewPath("/some/other/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other/dir/terragrunt.hcl"),
					},
					{
						Path:       project.NewPath("/some/other2/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other2/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "leaf module including root",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("include",
						Expr("path", `find_in_parent_folders()`),
					),
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
				).String(),
				`f:terragrunt.hcl:` + Block("locals",
					Str("a", "test"),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/terragrunt.hcl"),
						},
					},
				},
			},
		},
		{
			name: "leaf module including multiple files",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("include",
						Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),

					Block("include",
						Labels("file1"),
						Expr("path", `find_in_parent_folders("file1.hcl")`),
					),

					Block("include",
						Labels("file2"),
						Expr("path", `find_in_parent_folders("file2.hcl")`),
					),

					Block("include",
						Labels("file2"),
						Expr("path", `find_in_parent_folders("file2.hcl")`),
					),

					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
				).String(),
				`f:some/file1.hcl:` + Block("locals").String(),
				`f:some/file2.hcl:` + Block("locals").String(),
				`f:terragrunt.hcl:` + Block("locals",
					Str("a", "test"),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/some/file1.hcl"),
							project.NewPath("/some/file2.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
				},
			},
		},
		{
			name: "module including root that read files from locals",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("include", Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
				).String(),
				`f:terragrunt.hcl:` + Block("locals",
					Expr("a", `read_terragrunt_config(find_in_parent_folders("other.hcl"))`),
				).String(),
				`f:other.hcl:`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/other.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
				},
			},
		},
		{
			name: "module reading other config file",
			layout: []string{
				`f:terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("locals",
						Expr("a", `read_terragrunt_config("cfg1.hcl")`),
						Expr("b", `read_terragrunt_config("cfg2.hcl")`),
					),
				).String(),
				`f:cfg1.hcl:`,
				`f:cfg2.hcl:`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/cfg1.hcl"),
							project.NewPath("/cfg2.hcl"),
						},
					},
				},
			},
		},
		{
			name: "module reading tfvars",
			layout: []string{
				`f:terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("locals",
						Expr("vars", `read_tfvars_file("common.tfvars")`),
					),
				).String(),
				`f:common.tfvars:hello = "world"`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/common.tfvars"),
						},
					},
				},
			},
		},
		{
			name: "module reading file",
			layout: []string{
				`f:abc/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("locals",
						Expr("hello", `file("../hello.txt")`),
					),
				).String(),
				`f:hello.txt:world`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/abc"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/abc/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/hello.txt"),
						},
					},
				},
			},
		},
		{
			name: "module reading non-existent file",
			layout: []string{
				`f:abc/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					),
					Block("locals",
						Expr("hello", `try(file("../hello.txt"), "whatever")`),
					),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/abc"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/abc/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/hello.txt"),
						},
					},
				},
			},
		},
		{
			name: "local module directory also tracked as dependency",
			layout: []string{
				`f:some/dir/terragrunt.hcl:` + Doc(
					Block("terraform",
						Str("source", "${get_repo_root()}/modules/some"),
					),
					Block("include",
						Labels("root"),
						Expr("path", `find_in_parent_folders()`),
					),
					Block("dependency",
						Labels("other2"), // other2 declared before other: result must be sorted.
						Str("config_path", "../other2/dir"),
					),
					Block("dependency",
						Labels("other"),
						Str("config_path", "../other/dir"),
					),
				).String(),
				`f:modules/some/main.tf:# empty file`,
				`f:some/other/dir/terragrunt.hcl:` + Doc(
					Bool("skip", true),
					Block("terraform",
						Str("source", "https://some.etc/prj"),
					)).String(),
				`f:some/other2/dir/terragrunt.hcl:` + Block("terraform",
					Str("source", "https://some.etc/prj"),
				).String(),
				`f:terragrunt.hcl:` + Doc(
					Block("terraform"),
				).String(),
				`f:common.tfvars:a = "1"`,
				`f:regional.tfvars:b = "2"`,
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/some/dir"),
						Source:     "../../modules/some",
						ConfigFile: project.NewPath("/some/dir/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
							project.NewPath("/terragrunt.hcl"),
						},
						After: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
						DependencyBlocks: project.Paths{
							project.NewPath("/some/other/dir"),
							project.NewPath("/some/other2/dir"),
						},
					},
					{
						Path:       project.NewPath("/some/other/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other/dir/terragrunt.hcl"),
					},
					{
						Path:       project.NewPath("/some/other2/dir"),
						Source:     "https://some.etc/prj",
						ConfigFile: project.NewPath("/some/other2/dir/terragrunt.hcl"),
					},
				},
			},
		},
		{
			name: "nested Terraform modules",
			layout: []string{
				`f:terragrunt/terragrunt.hcl:` + Doc(
					Block("remote_state",
						Expr("generate", `{"path": "backend.tf", if_exists: "overwrite_terragrunt"}`),
					),
					Block("backend",
						Labels("provider"),
						Str("path", "provider.tf"),
						Str("if_exists", "overwrite_terragrunt"),
						Str("content", "a"),
					),
				).String(),
				`f:terragrunt/dev/a1/b1/terragrunt.hcl:` + Doc(
					Terraform(
						Str("source", "https://some.etc/prj"),
					),
					Expr("inputs", `[]`),
				).String(),
				`f:terragrunt/dev/a2/b2/c2_1/d2/terragrunt.hcl:` + Doc(
					Terraform(
						Str("source", "https://some.etc/prj"),
					),
					Block("dependencies",
						Expr("paths", `["../../../../a1/b1"]`),
					),
					Block("dependency", Labels("b1"),
						Str("config_path", "../../../../a1/b1"),
						Expr("mock_outputs_allowed_terraform_commands", `["validate", "plan", "refresh"]`),
						Str("mock_outputs_merge_strategy_with_state", "shallow"),
					),
				).String(),
				`f:terragrunt/dev/a2/b2/c2_2/terragrunt.hcl:` + Doc(
					Terraform(
						Str("source", "https://some.etc/prj"),
					),
					Block("dependencies",
						Expr("paths", `["../c2_3", "../../../a1/b1"]`),
					),
					Block("dependency", Labels("c2_3"),
						Str("config_path", "../c2_3"),
						Expr("mock_outputs_allowed_terraform_commands", `["validate", "plan", "refresh"]`),
						Str("mock_outputs_merge_strategy_with_state", "shallow"),
					),
					Expr("inputs", `[]`),
				).String(),
				`f:terragrunt/dev/a2/b2/c2_3/terragrunt.hcl:` + Doc(
					Terraform(
						Str("source", "https://some.etc/prj"),
					),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/terragrunt/dev/a1/b1"),
						ConfigFile: project.NewPath("/terragrunt/dev/a1/b1/terragrunt.hcl"),
						Source:     "https://some.etc/prj",
					},
					{
						Path:       project.NewPath("/terragrunt/dev/a2/b2/c2_1/d2"),
						ConfigFile: project.NewPath("/terragrunt/dev/a2/b2/c2_1/d2/terragrunt.hcl"),
						Source:     "https://some.etc/prj",
						After: project.Paths{
							project.NewPath("/terragrunt/dev/a1/b1"),
						},
						DependsOn: project.Paths{
							project.NewPath("/terragrunt/dev/a1/b1"),
						},
						DependencyBlocks: project.Paths{
							project.NewPath("/terragrunt/dev/a1/b1"),
						},
					},
					{
						Path:       project.NewPath("/terragrunt/dev/a2/b2/c2_2"),
						ConfigFile: project.NewPath("/terragrunt/dev/a2/b2/c2_2/terragrunt.hcl"),
						Source:     "https://some.etc/prj",
						After: project.Paths{
							project.NewPath("/terragrunt/dev/a1/b1"),
							project.NewPath("/terragrunt/dev/a2/b2/c2_3"),
						},
						DependsOn: project.Paths{
							project.NewPath("/terragrunt/dev/a2/b2/c2_3"),
						},
						DependencyBlocks: project.Paths{
							project.NewPath("/terragrunt/dev/a2/b2/c2_3"),
						},
					},
					{
						Path:       project.NewPath("/terragrunt/dev/a2/b2/c2_3"),
						ConfigFile: project.NewPath("/terragrunt/dev/a2/b2/c2_3/terragrunt.hcl"),
						Source:     "https://some.etc/prj",
					},
				},
			},
		},
		{
			name:   "terragrunt-live-example",
			layout: testInfraLayout(),
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/non-prod/us-east-1/qa/mysql"),
						ConfigFile: project.NewPath("/non-prod/us-east-1/qa/mysql/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/mysql?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/mysql.hcl"),
							project.NewPath("/non-prod/account.hcl"),
							project.NewPath("/non-prod/us-east-1/qa/env.hcl"),
							project.NewPath("/non-prod/us-east-1/region.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/non-prod/us-east-1/qa/webserver-cluster"),
						ConfigFile: project.NewPath("/non-prod/us-east-1/qa/webserver-cluster/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/asg-alb-service?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/webserver-cluster.hcl"),
							project.NewPath("/non-prod/account.hcl"),
							project.NewPath("/non-prod/us-east-1/qa/env.hcl"),
							project.NewPath("/non-prod/us-east-1/region.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/non-prod/us-east-1/stage/mysql"),
						ConfigFile: project.NewPath("/non-prod/us-east-1/stage/mysql/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/mysql?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/mysql.hcl"),
							project.NewPath("/non-prod/account.hcl"),
							project.NewPath("/non-prod/us-east-1/region.hcl"),
							project.NewPath("/non-prod/us-east-1/stage/env.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/non-prod/us-east-1/stage/webserver-cluster"),
						ConfigFile: project.NewPath("/non-prod/us-east-1/stage/webserver-cluster/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/asg-alb-service?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/webserver-cluster.hcl"),
							project.NewPath("/non-prod/account.hcl"),
							project.NewPath("/non-prod/us-east-1/region.hcl"),
							project.NewPath("/non-prod/us-east-1/stage/env.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/prod/us-east-1/prod/mysql"),
						ConfigFile: project.NewPath("/prod/us-east-1/prod/mysql/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/mysql?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/mysql.hcl"),
							project.NewPath("/prod/account.hcl"),
							project.NewPath("/prod/us-east-1/prod/env.hcl"),
							project.NewPath("/prod/us-east-1/region.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/prod/us-east-1/prod/webserver-cluster"),
						ConfigFile: project.NewPath("/prod/us-east-1/prod/webserver-cluster/terragrunt.hcl"),
						Source:     "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/asg-alb-service?ref=v0.8.0",
						DependsOn: project.Paths{
							project.NewPath("/_envcommon/webserver-cluster.hcl"),
							project.NewPath("/prod/account.hcl"),
							project.NewPath("/prod/us-east-1/prod/env.hcl"),
							project.NewPath("/prod/us-east-1/region.hcl"),
							project.NewPath("/terragrunt.hcl"),
						},
					},
				},
			},
		},
		{
			name: "nested includes with transitive dependencies through base/eks",
			layout: []string{
				// base/eks/terragrunt.hcl is just a shared config file (not runnable)
				`f:base/eks/terragrunt.hcl:` + Block("locals",
					Str("environment", "base"),
				).String(),

				// dev1/eks/terragrunt.hcl includes base/eks/terragrunt.hcl
				`f:dev1/eks/terragrunt.hcl:` + Doc(
					Block("include",
						Labels("base"),
						Expr("path", `find_in_parent_folders("base/eks/terragrunt.hcl")`),
					),
					Block("terraform",
						Str("source", "https://some.etc/eks"),
					),
				).String(),

				// prod/eks/terragrunt.hcl includes base/eks/terragrunt.hcl
				`f:prod/eks/terragrunt.hcl:` + Doc(
					Block("include",
						Labels("base"),
						Expr("path", `find_in_parent_folders("base/eks/terragrunt.hcl")`),
					),
					Block("terraform",
						Str("source", "https://some.etc/eks"),
					),
				).String(),

				// preprod/eks/terragrunt.hcl includes base/eks/terragrunt.hcl
				`f:preprod/eks/terragrunt.hcl:` + Doc(
					Block("include",
						Labels("base"),
						Expr("path", `find_in_parent_folders("base/eks/terragrunt.hcl")`),
					),
					Block("terraform",
						Str("source", "https://some.etc/eks"),
					),
				).String(),

				// base/foo/instance.hcl is an include file (not runnable, no terraform block)
				// It has dependencies on the three eks stacks
				// Paths are relative to the including file's directory
				`f:base/foo/instance.hcl:` + Doc(
					Block("dependency",
						Labels("dev1_eks"),
						Str("config_path", "../dev1/eks"),
					),
					Block("dependency",
						Labels("prod_eks"),
						Str("config_path", "../prod/eks"),
					),
					Block("dependency",
						Labels("preprod_eks"),
						Str("config_path", "../preprod/eks"),
					),
				).String(),

				// foo/terragrunt.hcl includes base/foo/instance.hcl and is runnable
				`f:foo/terragrunt.hcl:` + Doc(
					Block("include",
						Labels("base"),
						Expr("path", `find_in_parent_folders("base/foo/instance.hcl")`),
					),
					Block("terraform",
						Str("source", "https://some.etc/instance"),
					),
				).String(),
			},
			want: want{
				modules: tg.Modules{
					{
						Path:       project.NewPath("/dev1/eks"),
						Source:     "https://some.etc/eks",
						ConfigFile: project.NewPath("/dev1/eks/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/base/eks/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/foo"),
						Source:     "https://some.etc/instance",
						ConfigFile: project.NewPath("/foo/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/base/foo/instance.hcl"),
							project.NewPath("/dev1/eks"),
							project.NewPath("/preprod/eks"),
							project.NewPath("/prod/eks"),
						},
						After: project.Paths{
							project.NewPath("/dev1/eks"),
							project.NewPath("/preprod/eks"),
							project.NewPath("/prod/eks"),
						},
						DependencyBlocks: project.Paths{
							project.NewPath("/dev1/eks"),
							project.NewPath("/preprod/eks"),
							project.NewPath("/prod/eks"),
						},
					},
					{
						Path:       project.NewPath("/preprod/eks"),
						Source:     "https://some.etc/eks",
						ConfigFile: project.NewPath("/preprod/eks/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/base/eks/terragrunt.hcl"),
						},
					},
					{
						Path:       project.NewPath("/prod/eks"),
						Source:     "https://some.etc/eks",
						ConfigFile: project.NewPath("/prod/eks/terragrunt.hcl"),
						DependsOn: project.Paths{
							project.NewPath("/base/eks/terragrunt.hcl"),
						},
					},
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tc.layout)
			basedir := tc.basedir
			if basedir.String() == "" {
				basedir = project.NewPath("/")
			}
			modules, err := tg.ScanModules(s.RootDir(), basedir, !tc.ignoreDeps, nil)
			errtest.Assert(t, err, tc.want.err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(modules, tc.want.modules,
				cmpopts.IgnoreFields(tg.Module{}, "FilesProcessed"),
				cmpopts.EquateComparable(project.Path{}), cmpopts.IgnoreUnexported(tg.Module{})); diff != "" {
				t.Errorf("Diff (want [+], got [-]): %s", diff)
			}
		})
	}
}

func testInfraLayout() []string {
	return []string{
		// Root terragrunt.hcl
		"f:terragrunt.hcl:" + Doc(
			Block("locals",
				Expr("account_vars", `read_terragrunt_config(find_in_parent_folders("account.hcl"))`),
				Expr("region_vars", `read_terragrunt_config(find_in_parent_folders("region.hcl"))`),
				Expr("environment_vars", `read_terragrunt_config(find_in_parent_folders("env.hcl"))`),
				Expr("account_name", `local.account_vars.locals.account_name`),
				Expr("account_id", `local.account_vars.locals.aws_account_id`),
				Expr("aws_region", `local.region_vars.locals.aws_region`),
			),
			Block("remote_state",
				Str("backend", "s3"),
				Block("config",
					Bool("encrypt", true),
					Expr("bucket", `"${get_env("TG_BUCKET_PREFIX", "")}terragrunt-example-tf-state-${local.account_name}-${local.aws_region}"`),
					Str("key", "${path_relative_to_include()}/tf.tfstate"),
					Expr("region", "local.aws_region"),
					Str("dynamodb_table", "tf-locks"),
				),
				Block("generate",
					Str("path", "backend.tf"),
					Str("if_exists", "overwrite_terragrunt"),
				),
			),
			Block("catalog",
				Expr("urls", `[
  "https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example",
  "https://github.com/gruntwork-io/terraform-aws-utilities",
  "https://github.com/gruntwork-io/terraform-kubernetes-namespace"
]`),
			),
			Expr("inputs", `merge(
  local.account_vars.locals,
  local.region_vars.locals,
  local.environment_vars.locals,
)`),
		).String(),

		"f:_envcommon/mysql.hcl:" + Doc(
			Block("locals",
				Expr("environment_vars", `read_terragrunt_config(find_in_parent_folders("env.hcl"))`),
				Expr("env", `local.environment_vars.locals.environment`),
				Str("base_source_url", "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/mysql"),
			),
			Expr("inputs", `{
  name              = "mysql_${local.env}"
  instance_class    = "db.t3.micro"
  allocated_storage = 40
  storage_type      = "standard"
  master_username   = "admin"
}`),
		).String(),
		"f:_envcommon/webserver-cluster.hcl:" + Doc(
			Block("locals",
				Expr("environment_vars", `read_terragrunt_config(find_in_parent_folders("env.hcl"))`),
				Expr("env", `local.environment_vars.locals.environment`),
				Str("base_source_url", "git::https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example.git//modules/asg-alb-service"),
			),
			Expr("inputs", `{
  name          = "webserver-example-${local.env}"
  instance_type = "t2.micro"

  min_size = 0
  max_size = 1

  server_port = 8080
  alb_port    = 80
}`),
		).String(),

		"f:non-prod/account.hcl:" + Block("locals",
			Str("account_name", "non-prod"),
			Str("aws_account_id", "123456789012"),
		).String(),
		"f:non-prod/us-east-1/region.hcl:" + Block("locals",
			Str("aws_region", "us-east-1"),
		).String(),
		"f:non-prod/us-east-1/qa/env.hcl:" + Block("locals",
			Str("environment", "qa"),
		).String(),
		"f:non-prod/us-east-1/qa/mysql/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/mysql.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
		).String(),
		"f:non-prod/us-east-1/qa/webserver-cluster/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/webserver-cluster.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
		).String(),

		"f:non-prod/us-east-1/stage/env.hcl:" + Block("locals",
			Str("environment", "stage"),
		).String(),
		"f:non-prod/us-east-1/stage/mysql/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/mysql.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
		).String(),
		"f:non-prod/us-east-1/stage/webserver-cluster/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/webserver-cluster.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
		).String(),

		"f:prod/account.hcl:" + Block("locals",
			Str("account_name", "prod"),
			Str("aws_account_id", "987654321098"),
		).String(),

		"f:prod/us-east-1/region.hcl:" + Block("locals",
			Str("aws_region", "us-east-1"),
		).String(),

		"f:prod/us-east-1/prod/env.hcl:" + Block("locals",
			Str("environment", "prod"),
		).String(),
		"f:prod/us-east-1/prod/mysql/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/mysql.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
			Block("inputs",
				Str("instance_type", "db.t3.large"),
				Number("allocated_storage", 120),
			),
		).String(),
		"f:prod/us-east-1/prod/webserver-cluster/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("include",
				Labels("envcommon"),
				Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/webserver-cluster.hcl"`),
				Bool("expose", true),
			),
			Block("terraform",
				Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v0.8.0"`),
			),
			Block("inputs",
				Str("instance_type", "t3.medium"),
				Number("min_size", 0),
				Number("max_size", 2),
			),
		).String(),
	}
}
