// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"testing"

	"github.com/terramate-io/terramate/project"

	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

func BenchmarkModuleDiscovery(b *testing.B) {
	// mimics the repository below for a real world scenario
	// https://github.com/terramate-io/terramate-terragrunt-infrastructure-live-example/

	b.StopTimer()

	s := sandbox.NoGit(b, false)
	s.BuildTree(testInfraLayout())

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), true, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
