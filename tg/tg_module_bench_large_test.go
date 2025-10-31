// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

// BenchmarkLargeScaleWithCache benchmarks module discovery with cache on a large repository.
// This creates a realistic large-scale setup with:
// - 5 accounts
// - 5 regions per account
// - 4 environments per region
// - 8 services per environment
// Total: 5 * 5 * 4 * 8 = 800 modules
//
// Each module includes shared files, demonstrating significant cache reuse:
// - Root terragrunt.hcl (used by all 800 modules)
// - Account configs (5 files, each used by ~160 modules)
// - Region configs (25 files, each used by ~32 modules)
// - Environment configs (100 files, each used by ~8 modules)
func BenchmarkLargeScaleWithCache(b *testing.B) {
	b.StopTimer()

	s := sandbox.NoGit(b, false)
	layout := generateLargeScaleLayout(5, 5, 4, 8)

	b.Logf("Generated %d files for large-scale benchmark", len(layout))
	s.BuildTree(layout)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		modules, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), true)
		if err != nil {
			b.Fatal(err)
		}
		if i == 0 {
			b.Logf("Found %d modules", len(modules))
		}
	}
}

// BenchmarkLargeScaleWithoutCache benchmarks the same setup without cache sharing.
// This simulates the old behavior where each module parses all files independently.
func BenchmarkLargeScaleWithoutCache(b *testing.B) {
	b.StopTimer()

	s := sandbox.NoGit(b, false)

	accounts := 5
	regions := 5
	environments := 4
	services := 8

	layout := generateLargeScaleLayout(accounts, regions, environments, services)
	b.Logf("Generated %d files for large-scale benchmark", len(layout))
	s.BuildTree(layout)

	// Build list of all module locations
	var moduleFiles []struct {
		dir  project.Path
		file string
	}

	for acct := 1; acct <= accounts; acct++ {
		for region := 1; region <= regions; region++ {
			for env := 1; env <= environments; env++ {
				for svc := 1; svc <= services; svc++ {
					serviceNames := []string{"api", "web", "worker", "db", "cache", "queue", "storage", "monitor"}
					dir := fmt.Sprintf("/account-%d/region-%d/env-%d/%s", acct, region, env, serviceNames[svc-1])
					moduleFiles = append(moduleFiles, struct {
						dir  project.Path
						file string
					}{
						dir:  project.NewPath(dir),
						file: "terragrunt.hcl",
					})
				}
			}
		}
	}

	b.Logf("Found %d modules", len(moduleFiles))

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// Load each module independently without cache sharing (simulates old behavior)
		for _, mf := range moduleFiles {
			_, _, err := tg.LoadModule(s.RootDir(), mf.dir, mf.file, true)
			if err != nil {
				b.Fatalf("Failed to load module %s/%s: %v", mf.dir, mf.file, err)
			}
		}
	}
}

// generateLargeScaleLayout creates a large-scale Terragrunt repository layout.
// Structure: accounts / regions / environments / services
func generateLargeScaleLayout(accounts, regions, environments, services int) []string {
	var layout []string

	// Root terragrunt.hcl - included by ALL modules
	layout = append(layout,
		"f:terragrunt.hcl:"+Doc(
			Block("locals",
				Expr("account_vars", `read_terragrunt_config(find_in_parent_folders("account.hcl"))`),
				Expr("region_vars", `read_terragrunt_config(find_in_parent_folders("region.hcl"))`),
				Expr("environment_vars", `read_terragrunt_config(find_in_parent_folders("env.hcl"))`),
				Expr("account_id", `local.account_vars.locals.account_id`),
				Expr("region", `local.region_vars.locals.region`),
				Expr("environment", `local.environment_vars.locals.environment`),
			),
			Block("remote_state",
				Str("backend", "s3"),
				Block("config",
					Bool("encrypt", true),
					Expr("bucket", `"terraform-state-${local.account_id}-${local.region}"`),
					Str("key", "${path_relative_to_include()}/terraform.tfstate"),
					Expr("region", "local.region"),
				),
			),
			Expr("inputs", `merge(
  local.account_vars.locals,
  local.region_vars.locals,
  local.environment_vars.locals,
)`),
		).String(),
	)

	// Generate _envcommon files (one per service type)
	serviceNames := []string{"api", "web", "worker", "db", "cache", "queue", "storage", "monitor"}
	for _, svcName := range serviceNames {
		layout = append(layout,
			fmt.Sprintf("f:_envcommon/%s.hcl:", svcName)+Doc(
				Block("locals",
					Expr("environment_vars", `read_terragrunt_config(find_in_parent_folders("env.hcl"))`),
					Expr("env", `local.environment_vars.locals.environment`),
					Str("base_source_url", fmt.Sprintf("git::https://github.com/example/modules.git//services/%s", svcName)),
				),
				Expr("inputs", `{
  service_name = "`+svcName+`-${local.env}"
  instance_type = "t3.micro"
  replicas = 2
}`),
			).String(),
		)
	}

	// Generate account / region / environment / service hierarchy
	for acct := 1; acct <= accounts; acct++ {
		acctDir := fmt.Sprintf("account-%d", acct)

		// Account config
		layout = append(layout,
			fmt.Sprintf("f:%s/account.hcl:", acctDir)+Block("locals",
				Str("account_name", fmt.Sprintf("account-%d", acct)),
				Str("account_id", fmt.Sprintf("%012d", acct*111111111111)),
			).String(),
		)

		for region := 1; region <= regions; region++ {
			regionDir := fmt.Sprintf("%s/region-%d", acctDir, region)

			// Region config
			layout = append(layout,
				fmt.Sprintf("f:%s/region.hcl:", regionDir)+Block("locals",
					Str("region", fmt.Sprintf("us-east-%d", region)),
					Expr("availability_zones", fmt.Sprintf(`["us-east-%da", "us-east-%db"]`, region, region)),
				).String(),
			)

			for env := 1; env <= environments; env++ {
				envDir := fmt.Sprintf("%s/env-%d", regionDir, env)
				envNames := []string{"dev", "staging", "qa", "prod"}

				// Environment config
				layout = append(layout,
					fmt.Sprintf("f:%s/env.hcl:", envDir)+Block("locals",
						Str("environment", envNames[env-1]),
						Str("log_level", map[int]string{1: "debug", 2: "info", 3: "info", 4: "warn"}[env]),
					).String(),
				)

				// Services in this environment
				for svc := 1; svc <= services; svc++ {
					svcName := serviceNames[svc-1]
					svcDir := fmt.Sprintf("%s/%s", envDir, svcName)

					// Service module
					layout = append(layout,
						fmt.Sprintf("f:%s/terragrunt.hcl:", svcDir)+Doc(
							Block("include",
								Labels("root"),
								Expr("path", `find_in_parent_folders()`),
							),
							Block("include",
								Labels("envcommon"),
								Expr("path", `"${dirname(find_in_parent_folders())}/_envcommon/`+svcName+`.hcl"`),
								Bool("expose", true),
							),
							Block("terraform",
								Expr("source", `"${include.envcommon.locals.base_source_url}?ref=v1.0.0"`),
							),
						).String(),
					)
				}
			}
		}
	}

	return layout
}
