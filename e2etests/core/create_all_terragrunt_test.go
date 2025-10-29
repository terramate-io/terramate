// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test/hclwrite"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

func TestCreateAllTerragrunt(t *testing.T) {
	type testcase struct {
		name      string
		layout    []string
		tags      []string
		wd        string
		want      RunExpected
		wantOrder []string
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
				StderrRegex: string(tg.ErrParsing),
				Status:      1,
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
			wantOrder: []string{"."},
		},
		{
			name: "terragrunt modules with tags",
			layout: []string{
				hclfile("stacks/a/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
				hclfile("stacks/b/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
				hclfile("stacks/c/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			tags: []string{"terragrunt", "terraform"},
			want: RunExpected{
				Stdout: nljoin("Created stack /stacks/a", "Created stack /stacks/b", "Created stack /stacks/c"),
			},
			wantOrder: []string{"stacks/a", "stacks/b", "stacks/c"},
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
			wantOrder: []string{"dir"},
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
			wantOrder: []string{"dir"},
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
			wantOrder: []string{"mod1", "mod2", "mod3"},
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
			wantOrder: []string{"mod1", "mod1/mod2", "mod1/mod2/mod3"},
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
			wantOrder: []string{".", "dir"},
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
			wantOrder: []string{
				"prod/stacks/stack1",
				"prod/stacks/stack1/tg-stack",
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
			wantOrder: []string{"stack"},
		},
		{
			name: "terragrunt module at /A with dependencies at /modules",
			layout: []string{
				hclfile("A/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../modules/B"]`),
					),
				)),
				hclfile("modules/B/terragrunt.hcl", Block("terraform",
					Str("source", "github.com/some/repo"),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /A",
					"Created stack /modules/B",
				),
			},
			wantOrder: []string{"modules/B", "A"},
		},
		{
			name: "multiple orderings using dependencies block",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../2"]`),
					),
				)),
				hclfile("2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../3"]`),
					))),
				hclfile("3/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					))),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /1",
					"Created stack /2",
					"Created stack /3",
				),
			},
			wantOrder: []string{"3", "2", "1"},
		},
		{
			name: "multiple orderings using dependency block",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module2"),
						Str("config_path", `../2`),
					),
				)),
				hclfile("2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module3"),
						Str("config_path", `../3`),
					))),
				hclfile("3/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					))),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /1",
					"Created stack /2",
					"Created stack /3",
				),
			},
			wantOrder: []string{"3", "2", "1"},
		},
		{
			name: "multiple orderings using dependencies/dependency block",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module2"),
						Str("config_path", `../2`),
					),
					Block("dependencies",
						Expr("paths", `["../3"]`),
					))),
				hclfile("2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module3"),
						Str("config_path", `../3`),
					))),
				hclfile("3/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					))),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /1",
					"Created stack /2",
					"Created stack /3",
				),
			},
			wantOrder: []string{"3", "2", "1"},
		},
		{
			name: "nested terragrunt modules",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
				hclfile("1/2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module3"),
						Str("config_path", `..`),
					),
				),
				),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /1",
					"Created stack /1/2",
				),
			},
			wantOrder: []string{"1", "1/2"},
		},
		{
			name: "nested terragrunt modules conflicting with Terramate filesystem ordering",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module3"),
						Str("config_path", `./2`),
					),
				),
				),
				hclfile("1/2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
			},
			want: RunExpected{
				Status:      1,
				StderrRegex: regexp.QuoteMeta(`Module "/1/2" is defined as a child of the module stack it depends on`),
			},
			wantOrder: []string{"1", "1/2"},
		},
		{
			name: "nested terragrunt modules with sharing same prefix path",
			layout: []string{
				hclfile("A/foo/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module3"),
						Str("config_path", `../foobar`),
					),
				),
				),
				hclfile("A/foobar/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /A/foo",
					"Created stack /A/foobar",
				),
			},
			wantOrder: []string{"A/foobar", "A/foo"},
		},
		{
			name: "Terragrunt definitions with ciclic ordering",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../2"]`),
					),
				)),
				hclfile("2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../1"]`),
					))),
			},
			want: RunExpected{
				Status:      1,
				StderrRegex: "Found a dependency cycle between modules",
			},
		},
		{
			name: "Terragrunt with repeated depenendencies",
			layout: []string{
				hclfile("1/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependencies",
						Expr("paths", `["../2", "../2"]`),
					),
				)),
				hclfile("2/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
			},
			want: RunExpected{
				Stdout: nljoin(
					"Created stack /1",
					"Created stack /2",
				),
			},
			wantOrder: []string{"2", "1"},
		},
		{
			name: "Terragrunt with dependency defined outside of the Terramate project",
			layout: []string{
				hclfile("terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module2"),
						Str("config_path", `../other`),
					),
					Block("dependencies",
						Expr("paths", `["../other"]`),
					))),
				hclfile("../other/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
			},
			want: RunExpected{
				StderrRegexes: []string{
					regexp.QuoteMeta("Warning: Dependency outside of Terramate project detected in `dependency.config_path` configuration. Ignoring."),
					regexp.QuoteMeta("Warning: Dependency outside of Terramate project detected in `dependencies.paths` configuration. Ignoring."),
				},
				Stdout: nljoin(
					"Created stack /",
				),
			},
			wantOrder: []string{"."},
		},
		{
			name: "Terragrunt with dependency defined outside of the Terramate project (sharing same base path)",
			layout: []string{
				hclfile("terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
					Block("dependency",
						Labels("module2"),
						Str("config_path", `../sandboxy`),
					),
					Block("dependencies",
						Expr("paths", `["../sandboxy"]`),
					))),
				hclfile("../sandboxy/terragrunt.hcl", Doc(
					Block("terraform",
						Str("source", "github.com/some/repo"),
					),
				)),
			},
			want: RunExpected{
				StderrRegexes: []string{
					regexp.QuoteMeta("Warning: Dependency outside of Terramate project detected in `dependency.config_path` configuration. Ignoring."),
					regexp.QuoteMeta("Warning: Dependency outside of Terramate project detected in `dependencies.paths` configuration. Ignoring."),
				},
				Stdout: nljoin(
					"Created stack /",
				),
			},
			wantOrder: []string{"."},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)
			tm := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))
			args := []string{"create", "--all-terragrunt"}
			if len(tc.tags) > 0 {
				args = append(args, "--tags", strings.Join(tc.tags, ","))
			}
			res := tm.Run(args...)
			AssertRunResult(t,
				res,
				tc.want,
			)
			if res.Status == 0 {
				tm := NewCLI(t, filepath.Join(s.RootDir(), tc.wd))
				res := tm.Run("list", "--run-order")
				AssertRunResult(t, res, RunExpected{
					Stdout:       nljoin(tc.wantOrder...),
					IgnoreStderr: true, // Ignore warnings that may be re-emitted
				})

				if len(tc.tags) == 0 {
					return
				}

				// check if tags are present
				root := s.ReloadConfig()
				rootTree := root.Tree()
				if tc.wd != "" {
					if !filepath.IsAbs(tc.wd) {
						tc.wd = filepath.Join(s.RootDir(), tc.wd)
					}
					rootTree, _ = root.Lookup(project.PrjAbsPath(s.RootDir(), tc.wd))
				}

				assert.EqualInts(t, len(tc.wantOrder), len(rootTree.Stacks()))

				for _, tree := range rootTree.Stacks() {
					st, err := tree.Stack()
					assert.NoError(t, err)

					assert.EqualInts(t, len(tc.tags), len(st.Tags))

					for _, tag := range tc.tags {
						if !slices.Contains(st.Tags, tag) {
							t.Errorf("stack %s does not have tag %s", st.Dir, tag)
						}
					}
				}
			}
		})
	}
}
