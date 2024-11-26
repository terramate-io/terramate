// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

const testBranchName = "test-branch"

func prepareBranch(t *testing.T) *sandbox.S {
	s := sandbox.New(t)
	s.BuildTree([]string{
		"s:stacks/s1",
		"f:stacks/s1/main.tf:# main",
		"s:stacks/s2",
		"f:stacks/s2/main.tf:# main",
		"s:stacks/s3",
		"f:stacks/s3/main.tf:# main",
		"f:script.tm:" + Doc(
			Terramate(
				Config(
					Expr("experiments", `["scripts"]`),
				),
			),
			Script(
				Labels("test"),
				Block("job",
					Expr("command", fmt.Sprintf(`["%s", "stack-abs-path", "${tm_chomp(<<-EOF
	%s
EOF
)}"]`, HelperPathAsHCL, s.RootDir())),
				),
			),
		).String(),
	})
	s.Git().CommitAll("create stacks")
	s.Git().Push("main")
	s.Git().CheckoutNew(testBranchName)
	return &s
}

func TestChangeDetection(t *testing.T) {
	t.Parallel()
	// TODO(i4k): migrate all tests in manager_test.go to use the sandbox.

	t.Run("no config, no changes", func(t *testing.T) {
		t.Parallel()
		s := prepareBranch(t)
		mgr := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())
		report, err := mgr.ListChanged(stack.ChangeConfig{
			BaseRef: "origin/main",
		})
		assert.NoError(t, err)
		assert.EqualInts(t, len(report.Stacks), 0)
		assert.EqualInts(t, len(report.Checks.UntrackedFiles), 0)
		assert.EqualInts(t, len(report.Checks.UncommittedFiles), 0)

		tmcli := NewCLI(t, s.RootDir())
		AssertRun(t, tmcli.Run("list", "--changed"))
		AssertRun(t, tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked"))
		AssertRun(t, tmcli.Run("list", "--changed", "--disable-change-detection=git-uncommitted"))
		AssertRun(t, tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked,git-uncommitted"))

		AssertRun(t, tmcli.Run("run", "--changed", "--", HelperPath, "stack-abs-path", s.RootDir()))
		AssertRun(t, tmcli.Run("run", "--changed", "--enable-change-detection=git-untracked", "--", HelperPath, "stack-abs-path", s.RootDir()))
		AssertRun(t, tmcli.Run("run", "--changed", "--enable-change-detection=git-uncommitted", "--", HelperPath, "stack-abs-path", s.RootDir()))
	})

	t.Run("no config/single stack changed", func(t *testing.T) {
		t.Parallel()
		s := prepareBranch(t)
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s1"), "main.tf", "# changed")
		s.Git().CommitAll("s1 changed")
		mgr := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())
		report, err := mgr.ListChanged(stack.ChangeConfig{
			BaseRef: "origin/main",
		})
		assert.NoError(t, err)
		assert.EqualInts(t, len(report.Stacks), 1)
		assert.EqualInts(t, len(report.Checks.UntrackedFiles), 0)
		assert.EqualInts(t, len(report.Checks.UncommittedFiles), 0)
		assert.EqualStrings(t, report.Stacks[0].Stack.Dir.String(), "/stacks/s1")

		tmcli := NewCLI(t, s.RootDir())
		AssertRunResult(t,
			tmcli.Run("list", "--changed"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked,git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)
		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-untracked", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)
		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-uncommitted", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "test"),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--enable-change-detection=git-untracked", "test"),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--enable-change-detection=git-uncommitted", "test"),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)
	})

	t.Run("with config disabling all", func(t *testing.T) {
		t.Parallel()
		s := prepareBranch(t)
		s.BuildTree([]string{
			`f:change_detection.tm:` + Terramate(
				Config(
					Block("change_detection",
						Block("git",
							Str("untracked", "off"),
							Str("uncommitted", "off"),
						),
					),
				),
			).String(),
		})
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s1"), "main.tf", "# changed")
		s.Git().CommitAll("s1 changed")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s2"), "main.tf", "# uncommitted")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s3"), "untracked.tf", "# something")
		mgr := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())
		report, err := mgr.ListChanged(stack.ChangeConfig{
			BaseRef: "origin/main",
		})
		assert.NoError(t, err)
		assert.EqualInts(t, 1, len(report.Stacks))
		assert.EqualInts(t, 1, len(report.Checks.UntrackedFiles))
		assert.EqualInts(t, 1, len(report.Checks.UncommittedFiles))
		assert.EqualStrings(t, report.Stacks[0].Stack.Dir.String(), "/stacks/s1")

		tmcli := NewCLI(t, s.RootDir())
		AssertRunResult(t,
			tmcli.Run("list", "--changed"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--enable-change-detection=git-untracked"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s3")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--enable-change-detection=git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s2")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked,git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--enable-change-detection=git-uncommitted,git-untracked"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s2", "stacks/s3")},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has untracked files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-safeguards=git-untracked", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has uncommitted files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-change-detection=git-untracked", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-change-detection=git-uncommitted", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-untracked", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s3"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-uncommitted", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s2"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-uncommitted,git-untracked", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s2", "/stacks/s3"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "test"),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has untracked files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--disable-safeguards=git-untracked", "test"),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has uncommitted files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--disable-safeguards=git-uncommitted,git-untracked", "test"),
			RunExpected{Stdout: nljoin("/stacks/s1")},
		)
	})

	t.Run("with config enabling all", func(t *testing.T) {
		t.Parallel()
		s := prepareBranch(t)
		s.BuildTree([]string{
			`f:change_detection.tm:` + Terramate(
				Config(
					Block("change_detection",
						Block("git",
							Str("untracked", "on"),
							Str("uncommitted", "on"),
						),
					),
				),
			).String(),
		})
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s1"), "main.tf", "# changed")
		s.Git().CommitAll("s1 changed")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s2"), "main.tf", "# uncommitted")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s3"), "untracked.tf", "# something")
		mgr := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())
		report, err := mgr.ListChanged(stack.ChangeConfig{
			BaseRef: "origin/main",
		})
		assert.NoError(t, err)
		assert.EqualInts(t, 3, len(report.Stacks))
		assert.EqualInts(t, 1, len(report.Checks.UntrackedFiles))
		assert.EqualInts(t, 1, len(report.Checks.UncommittedFiles))
		assert.EqualStrings(t, "/stacks/s3/untracked.tf", report.Checks.UntrackedFiles[0].String())
		assert.EqualStrings(t, "/stacks/s2/main.tf", report.Checks.UncommittedFiles[0].String())
		assert.EqualStrings(t, report.Stacks[0].Stack.Dir.String(), "/stacks/s1")
		assert.EqualStrings(t, report.Stacks[1].Stack.Dir.String(), "/stacks/s2")
		assert.EqualStrings(t, report.Stacks[2].Stack.Dir.String(), "/stacks/s3")

		tmcli := NewCLI(t, s.RootDir())
		AssertRunResult(t,
			tmcli.Run("list", "--changed"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s2", "stacks/s3")},
		)

		// enabling in the CLI is a no-op because config has them enabled already.
		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--enable-change-detection=git-untracked,git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s2", "stacks/s3")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s2")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1", "stacks/s3")},
		)

		AssertRunResult(t,
			tmcli.Run("list", "--changed", "--disable-change-detection=git-untracked,git-uncommitted"),
			RunExpected{Stdout: nljoin("stacks/s1")},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has untracked files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-safeguards=git-untracked", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has uncommitted files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s2", "/stacks/s3"),
			},
		)

		// enabling in the CLI is a no-op because the config has them enabled already.
		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--enable-change-detection=git-untracked,git-uncommitted", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s2", "/stacks/s3"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-change-detection=git-uncommitted", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s3"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-change-detection=git-untracked", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1", "/stacks/s2"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-change-detection=git-untracked,git-uncommitted", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{
				Stdout: nljoin("/stacks/s1"),
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "test"),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has untracked files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--disable-safeguards=git-untracked", "test"),
			RunExpected{
				Status:      1,
				StderrRegex: "Error: repository has uncommitted files",
			},
		)

		AssertRunResult(t,
			tmcli.Run("script", "run", "--quiet", "--changed", "--disable-safeguards=git-uncommitted,git-untracked", "test"),
			RunExpected{Stdout: nljoin("/stacks/s1", "/stacks/s2", "/stacks/s3")},
		)
	})

	t.Run("trigger --ignore-change ignores stacks marked by dirty files", func(t *testing.T) {
		t.Parallel()
		s := prepareBranch(t)
		s.BuildTree([]string{
			`f:change_detection.tm:` + Terramate(
				Config(
					Block("change_detection",
						Block("git",
							Str("untracked", "on"),
							Str("uncommitted", "on"),
						),
					),
				),
			).String(),
		})
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s1"), "main.tf", "# changed")
		s.Git().CommitAll("s1 changed")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s2"), "main.tf", "# uncommitted")
		test.WriteFile(t, filepath.Join(s.RootDir(), "stacks/s3"), "untracked.tf", "# something")

		tmcli := NewCLI(t, s.RootDir())
		AssertRunResult(t,
			tmcli.Run("experimental", "trigger", "--ignore-change", "--recursive", "./stacks"),
			RunExpected{
				Stdout: nljoin(
					`Created ignore trigger for stack "/stacks/s1"`,
					`Created ignore trigger for stack "/stacks/s2"`,
					`Created ignore trigger for stack "/stacks/s3"`,
				),
			},
		)

		mgr := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())
		report, err := mgr.ListChanged(stack.ChangeConfig{
			BaseRef: "origin/main",
		})
		assert.NoError(t, err)
		assert.EqualInts(t, 0, len(report.Stacks))
		assert.EqualInts(t, 4, len(report.Checks.UntrackedFiles))
		assert.EqualInts(t, 1, len(report.Checks.UncommittedFiles))
		assert.EqualStrings(t, "/stacks/s3/untracked.tf", report.Checks.UntrackedFiles[0].String())
		if !strings.HasPrefix(report.Checks.UntrackedFiles[1].String(), "/.tmtriggers/stacks/s1/ignore-change-") {
			t.Errorf("unexpected untracked file: %s", report.Checks.UntrackedFiles[1])
		}
		if !strings.HasPrefix(report.Checks.UntrackedFiles[2].String(), "/.tmtriggers/stacks/s2/ignore-change-") {
			t.Errorf("unexpected untracked file: %s", report.Checks.UntrackedFiles[2])
		}
		if !strings.HasPrefix(report.Checks.UntrackedFiles[3].String(), "/.tmtriggers/stacks/s3/ignore-change-") {
			t.Errorf("unexpected untracked file: %s", report.Checks.UntrackedFiles[3])
		}
		assert.EqualStrings(t, "/stacks/s2/main.tf", report.Checks.UncommittedFiles[0].String())

		AssertRunResult(t,
			tmcli.Run("run", "--quiet", "--changed", "--disable-safeguards=git", "--", HelperPath, "stack-abs-path", s.RootDir()),
			RunExpected{},
		)
	})
}

func TestTriggerChangeDetection(t *testing.T) {
	t.Run("trigger --ignore + Terraform module changes", func(t *testing.T) {
		s := prepareBranch(t)
		s.BuildTree([]string{
			"f:modules/mod1/main.tf:# mod1 module",
			"f:stacks/s3/use_mod1.tf:" + Block("module",
				Labels("something"),
				Str("source", "../../modules/mod1"),
			).String(),
		})
		s.Git().CommitAll("commit module usage")
		s.Git().Checkout("main")
		s.Git().Merge(testBranchName)
		s.Git().Push("main")
		s.Git().DeleteBranch(testBranchName)
		s.Git().CheckoutNew(testBranchName)

		tmcli := NewCLI(t, s.RootDir())
		AssertRun(t, tmcli.Run("list", "--changed"))

		// change Terraform module
		test.WriteFile(t, filepath.Join(s.RootDir(), "modules/mod1"), "main.tf", "# changed")
		AssertRunResult(t, tmcli.Run("list", "--changed"), RunExpected{
			Stdout: nljoin("stacks/s3"),
		})

		AssertRunResult(t,
			tmcli.Run("experimental", "trigger", "--ignore-change", "./stacks/s3"),
			RunExpected{
				Stdout: nljoin(`Created ignore trigger for stack "/stacks/s3"`),
			},
		)

		AssertRun(t, tmcli.Run("list", "--changed"))
	})

	t.Run("trigger --ignore + Terragrunt module changes", func(t *testing.T) {
		s := prepareBranch(t)
		s.BuildTree([]string{
			"f:modules/mod1/main.tf:# mod1 module",
			"f:stacks/s3/terragrunt.hcl:" + Block("terraform",
				Str("source", "../../modules/mod1"),
			).String(),
		})
		s.Git().CommitAll("commit module usage")
		s.Git().Checkout("main")
		s.Git().Merge(testBranchName)
		s.Git().Push("main")
		s.Git().DeleteBranch(testBranchName)
		s.Git().CheckoutNew(testBranchName)

		tmcli := NewCLI(t, s.RootDir())
		AssertRun(t, tmcli.Run("list", "--changed"))

		// change Terraform module
		test.WriteFile(t, filepath.Join(s.RootDir(), "modules/mod1"), "main.tf", "# changed")
		AssertRunResult(t, tmcli.Run("list", "--changed"), RunExpected{
			Stdout: nljoin("stacks/s3"),
		})

		AssertRunResult(t,
			tmcli.Run("experimental", "trigger", "--ignore-change", "./stacks/s3"),
			RunExpected{
				Stdout: nljoin(`Created ignore trigger for stack "/stacks/s3"`),
			},
		)

		AssertRun(t, tmcli.Run("list", "--changed"))
	})
}
