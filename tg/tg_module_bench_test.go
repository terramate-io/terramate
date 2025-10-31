// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"testing"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

// BenchmarkModuleDiscoveryWithCache benchmarks module discovery with the new shared cache.
// This represents the optimized behavior where all modules in a scan share a parse cache,
// avoiding re-parsing of common files like root terragrunt.hcl, account.hcl, region.hcl, etc.
func BenchmarkModuleDiscoveryWithCache(b *testing.B) {
	// mimics the repository below for a real world scenario
	// https://github.com/terramate-io/terramate-terragrunt-infrastructure-live-example/

	b.StopTimer()

	s := sandbox.NoGit(b, false)
	s.BuildTree(testInfraLayout())

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), true)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkModuleDiscoveryWithoutCache benchmarks module discovery without cache sharing.
// This simulates the old behavior where each module loads independently with its own cache,
// similar to how parallel workers would behave without a shared cache.
// In a typical terragrunt repo with 6 modules that each include a root config:
// - root terragrunt.hcl gets parsed 6 times (once per module)
// - shared files like account.hcl, region.hcl get parsed multiple times
func BenchmarkModuleDiscoveryWithoutCache(b *testing.B) {
	b.StopTimer()

	s := sandbox.NoGit(b, false)
	s.BuildTree(testInfraLayout())

	// Find all actual module directories (ones with terraform.source)
	// These are in non-prod/us-east-1/{qa,stage}/{mysql,webserver-cluster}
	// and prod/us-east-1/prod/{mysql,webserver-cluster}
	moduleFiles := []struct {
		dir  project.Path
		file string
	}{
		{dir: project.NewPath("/non-prod/us-east-1/qa/mysql"), file: "terragrunt.hcl"},
		{dir: project.NewPath("/non-prod/us-east-1/qa/webserver-cluster"), file: "terragrunt.hcl"},
		{dir: project.NewPath("/non-prod/us-east-1/stage/mysql"), file: "terragrunt.hcl"},
		{dir: project.NewPath("/non-prod/us-east-1/stage/webserver-cluster"), file: "terragrunt.hcl"},
		{dir: project.NewPath("/prod/us-east-1/prod/mysql"), file: "terragrunt.hcl"},
		{dir: project.NewPath("/prod/us-east-1/prod/webserver-cluster"), file: "terragrunt.hcl"},
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// Load each module independently without cache sharing (simulates old behavior)
		for _, mf := range moduleFiles {
			_, _, err := tg.LoadModule(s.RootDir(), mf.dir, mf.file, true)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// Legacy alias for backward compatibility
func BenchmarkModuleDiscovery(b *testing.B) {
	BenchmarkModuleDiscoveryWithCache(b)
}
