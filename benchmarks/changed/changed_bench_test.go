// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package changed_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/rs/zerolog"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func BenchmarkChangeDetection(b *testing.B) {
	b.StopTimer()

	const nstacks = 10

	s := sandbox.New(b)
	layout := []string{
		"f:modules/common/common-1/main.tf:# nothing",
		"f:modules/common/common-2/main.tf:# nothing",
		"f:modules/common/common-3/main.tf:# nothing",
	}

	for i := 0; i < nstacks; i++ {
		layout = append(layout, fmt.Sprintf("f:modules/mod-%d/main.tf:%s", i, "# nothing here"))
		layout = append(layout, fmt.Sprintf("s:stack-%d", i))
		layout = append(layout, fmt.Sprintf("f:stack-%d/main.tf:%s", i, Doc(
			Block("module",
				Labels("something"),
				Str("source", fmt.Sprintf("../modules/mod-%d", i)),
			),
		).String()))

		// only even stacks has the common modules
		if i%2 == 0 {
			layout = append(layout, fmt.Sprintf("f:stack-%d/use-common.tf:%s", i, Doc(
				Block("module",
					Labels("common_mod1"),
					Str("source", "../modules/common/common-1"),
				),
				Block("module",
					Labels("common_mod2"),
					Str("source", "../modules/common/common-2"),
				),
				Block("module",
					Labels("common_mod3"),
					Str("source", "../modules/common/common-3"),
				),
			)))
		}
	}
	s.BuildTree(layout)
	s.Git().CommitAll("create repo")
	s.Git().Push("main")
	s.Git().CheckoutNew("modify-some-modules")
	test.WriteFile(b, filepath.Join(s.RootDir(), fmt.Sprintf("modules/mod-%d", nstacks-1)), "main.tf", "# modified")
	test.WriteFile(b, filepath.Join(s.RootDir(), "modules/common/common-3"), "main.tf", "# modified")
	s.Git().CommitAll("module modified")

	manager := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		report, err := manager.ListChanged(stack.ChangeConfig{BaseRef: "origin/main"})
		assert.NoError(b, err)
		assert.EqualInts(b, 6, len(report.Stacks))

		assert.EqualStrings(b, "/stack-0", report.Stacks[0].Stack.Dir.String())
		assert.EqualStrings(b, "/stack-2", report.Stacks[1].Stack.Dir.String())
		assert.EqualStrings(b, "/stack-4", report.Stacks[2].Stack.Dir.String())
		assert.EqualStrings(b, "/stack-6", report.Stacks[3].Stack.Dir.String())
		assert.EqualStrings(b, "/stack-8", report.Stacks[4].Stack.Dir.String())
		assert.EqualStrings(b, fmt.Sprintf("/stack-%d", nstacks-1), report.Stacks[5].Stack.Dir.String())
	}
}

func BenchmarkChangeDetectionTFAndTG(b *testing.B) {
	b.StopTimer()

	const nTfStacks = 10
	const nTGstacks = 10

	s := sandbox.New(b)
	layout := []string{
		"f:config.hcl:" + Block("locals",
			Expr("account", `read_terragrunt_config(find_in_parent_folders("account.hcl"))`),
		).String(),
		"f:account.hcl:" + Block("locals",
			Str("account_name", "prod"),
			Str("aws_account_id", "test"),
		).String(),
	}
	for i := 0; i < nTfStacks; i++ {
		layout = append(layout, fmt.Sprintf("f:modules/mod-%d/main.tf:%s", i, "# nothing here"))
		layout = append(layout, fmt.Sprintf("s:stack-%d", i))
		layout = append(layout, fmt.Sprintf("f:stack-%d/main.tf:%s", i, Block("module",
			Labels("something"),
			Str("source", fmt.Sprintf("../modules/mod-%d", i)),
		).String()))
	}
	for i := 0; i < nTGstacks; i++ {
		layout = append(layout, fmt.Sprintf("f:modules/mod-%d/main.tf:%s", i, "# nothing here"))
		layout = append(layout, fmt.Sprintf("s:tg-%d", i))
		layout = append(layout, fmt.Sprintf("f:tg-%d/terragrunt.hcl:%s", i, Block("terraform",
			Str("source", fmt.Sprintf("../modules/mod-%d", i)),
		).String()))
	}
	s.BuildTree(layout)
	s.Git().CommitAll("create repo")
	s.Git().Push("main")
	s.Git().CheckoutNew("modify-some-modules")
	test.WriteFile(b, filepath.Join(s.RootDir(), fmt.Sprintf("modules/mod-%d", nTfStacks-1)), "main.tf", "# modified")
	s.Git().CommitAll("module modified")

	manager := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		report, err := manager.ListChanged(stack.ChangeConfig{BaseRef: "origin/main"})
		assert.NoError(b, err)
		assert.EqualInts(b, 2, len(report.Stacks))
		assert.EqualStrings(b, fmt.Sprintf("/stack-%d", nTfStacks-1), report.Stacks[0].Stack.Dir.String())
		assert.EqualStrings(b, fmt.Sprintf("/tg-%d", nTGstacks-1), report.Stacks[1].Stack.Dir.String())
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
