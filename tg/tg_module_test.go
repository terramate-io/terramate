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
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, false)
			s.BuildTree(tc.layout)
			basedir := tc.basedir
			if basedir.String() == "" {
				basedir = project.NewPath("/")
			}
			modules, err := tg.ScanModules(s.RootDir(), basedir, !tc.ignoreDeps)
			errtest.Assert(t, err, tc.want.err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(modules, tc.want.modules, cmpopts.EquateComparable(project.Path{})); diff != "" {
				t.Errorf("Diff (want [+], got [-]): %s", diff)
			}
		})
	}
}
