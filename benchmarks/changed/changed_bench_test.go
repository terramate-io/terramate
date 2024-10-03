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
	layout := []string{}
	for i := 0; i < nstacks; i++ {
		layout = append(layout, fmt.Sprintf("f:modules/mod-%d/main.tf:%s", i, "# nothing here"))
		layout = append(layout, fmt.Sprintf("s:stack-%d", i))
		layout = append(layout, fmt.Sprintf("f:stack-%d/main.tf:%s", i, Block("module",
			Labels("something"),
			Str("source", fmt.Sprintf("../modules/mod-%d", i)),
		).String()))
	}
	s.BuildTree(layout)
	s.Git().CommitAll("create repo")
	s.Git().Push("main")
	s.Git().CheckoutNew("modify-some-modules")
	test.WriteFile(b, filepath.Join(s.RootDir(), fmt.Sprintf("modules/mod-%d", nstacks-1)), "main.tf", "# modified")
	s.Git().CommitAll("module modified")

	manager := stack.NewGitAwareManager(s.Config(), s.Git().Unwrap())

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		report, err := manager.ListChanged("origin/main")
		assert.NoError(b, err)
		assert.EqualInts(b, 1, len(report.Stacks))
		assert.EqualStrings(b, fmt.Sprintf("/stack-%d", nstacks-1), report.Stacks[0].Stack.Dir.String())
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
