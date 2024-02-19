// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCreateAllTerragrunt(t *testing.T) {
	type testcase struct {
		name   string
		layout []string
		wd     string
		want   RunExpected
	}

	hclfile := func(name string, content *hclwrite.Block) string {
		return "f:" + name + ":" + content.String()
	}

	for _, tc := range []testcase{
		{
			// maybe user's version of terragrunt is incompatible with Terramate implementation.
			name: "invalid terragrunt configuration",
			layout: []string{
				hclfile("terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
					Str("unknown_field", "test"),
				)),
			},
			want: RunExpected{
				Stderr: nljoin("Failed to detect bleh"),
			},
		},
		{
			name: "terragrunt module at root",
			layout: []string{
				hclfile("terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin("Created stack /"),
			},
		},
		{
			name: "terragrunt module at /dir but terraform.source imported from /_common",
			layout: []string{
				hclfile("dir/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../_common/module.hcl"),
				)),
				hclfile("_common/module.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: "Created stack /dir\n",
			},
		},
		{
			name: "terragrunt module at /dir merged with terraform block from root",
			layout: []string{
				hclfile("dir/terragrunt.hcl", Doc(
					Block("include",
						Labels("root"),
						Expr("path", "find_in_parent_folders()"),
					),
					Block("terraform",
						Str("source", "github.com/some/repo"),
					)),
				),
				hclfile("terragrunt.hcl", Block("terraform",
					Block("extra_arguments",
						Labels("common_vars"),
						Expr("commands", "get_terraform_commands_that_need_vars()"),
					)),
				),
			},
			want: RunExpected{
				Stdout: "Created stack /dir\n",
			},
		},
		{
			name: "multiple siblings terragrunt modules using same TF module",
			layout: []string{
				hclfile("mod1/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../_common/mod1.hcl"),
				)),
				hclfile("mod2/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../_common/mod1.hcl"),
				)),
				hclfile("mod3/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../_common/mod1.hcl"),
				)),
				hclfile("_common/mod1.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /mod1",
					"Created stack /mod2",
					"Created stack /mod3",
				),
			},
		},
		{
			name: "nested terragrunt modules using same TF module",
			layout: []string{
				hclfile("mod1/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../_common/mod.hcl"),
				)),
				hclfile("mod1/mod2/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../../_common/mod.hcl"),
				)),
				hclfile("mod1/mod2/mod3/terragrunt.hcl", Block("include",
					Labels("envcommon"),
					Str("path", "../../../_common/mod.hcl"),
				)),
				hclfile("_common/mod.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /mod1",
					"Created stack /mod1/mod2",
					"Created stack /mod1/mod2/mod3",
				),
			},
		},
		{
			name: "terragrunt module at /dir merged with terraform module from root",
			layout: []string{
				hclfile("dir/terragrunt.hcl", Block("include",
					Labels("root"),
					Expr("path", "find_in_parent_folders()"),
				)),
				hclfile("terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
					Block("extra_arguments",
						Labels("common_vars"),
						Expr("commands", "get_terraform_commands_that_need_vars()"),
					)),
				),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /",
					"Created stack /dir",
				),
			},
		},
		{
			name: "detects Terragrunt module inside Terramate stack",
			layout: []string{
				`s:prod/stacks/stack1`,
				hclfile("prod/stacks/stack1/tg-stack/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /prod/stacks/stack1/tg-stack",
				),
			},
		},
		{
			name: "respects working dir and only creates for child directories",
			wd:   "prod",
			layout: []string{
				hclfile("prod/stack/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
				hclfile("dev/stack/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /prod/stack",
				),
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)
			tm := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))
			AssertRunResult(t,
				tm.Run("create", "--all-terragrunt"),
				tc.want,
			)
		})
	}
}
