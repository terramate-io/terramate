// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tg"
)

// TestCacheAPI verifies the exported cache API works correctly
func TestCacheAPI(t *testing.T) {
	t.Parallel()

	// Test cache creation
	cache := tg.NewParseCache()
	if cache == nil {
		t.Fatal("NewParseCache returned nil")
	}

	// Test that we can create multiple caches
	cache2 := tg.NewParseCache()
	if cache2 == nil {
		t.Fatal("NewParseCache returned nil on second call")
	}

	// Verify they are different instances
	if cache == cache2 {
		t.Error("Multiple NewParseCache() calls returned same instance")
	}
}

// TestLoadModuleWithCache verifies LoadModuleWithCache works
func TestLoadModuleWithCache(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("terraform",
				Str("source", "git::https://example.com/module.git"),
			),
		).String(),
	})

	cache := tg.NewParseCache()

	// Load with cache - should succeed
	mod, isRoot, err := tg.LoadModuleWithCache(s.RootDir(), project.NewPath("/"), "terragrunt.hcl", false, cache)
	if err != nil {
		t.Fatalf("LoadModuleWithCache failed: %v", err)
	}

	if !isRoot {
		t.Error("Expected module to be a root module")
	}

	if mod == nil {
		t.Fatal("LoadModuleWithCache returned nil module")
	}

	if mod.Source != "git::https://example.com/module.git" {
		t.Errorf("Expected source 'git::https://example.com/module.git', got %q", mod.Source)
	}
}

// TestCacheConcurrency verifies thread-safe concurrent access to cache
func TestCacheConcurrency(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)

	// Create multiple simple modules
	var layout []string
	numModules := 20
	for i := 0; i < numModules; i++ {
		modPath := fmt.Sprintf("mod%d", i)
		layout = append(layout,
			"f:"+modPath+"/terragrunt.hcl:"+Doc(
				Block("terraform",
					Str("source", fmt.Sprintf("git::https://example.com/module%d.git", i)),
				),
			).String(),
		)
	}

	s.BuildTree(layout)

	cache := tg.NewParseCache()
	var wg sync.WaitGroup
	errors := make(chan error, numModules)
	successCount := make(chan int, numModules)

	// Load all modules concurrently
	for i := 0; i < numModules; i++ {
		wg.Add(1)
		go func(modNum int) {
			defer wg.Done()
			modPath := project.NewPath(fmt.Sprintf("/mod%d", modNum))
			_, isRoot, err := tg.LoadModuleWithCache(s.RootDir(), modPath, "terragrunt.hcl", false, cache)
			if err != nil {
				errors <- err
			} else if isRoot {
				successCount <- 1
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(successCount)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent load failed: %v", err)
	}

	// Count successful loads
	count := 0
	for range successCount {
		count++
	}

	if count != numModules {
		t.Errorf("Expected %d successful loads, got %d", numModules, count)
	}
}

// TestCacheWithScanModules verifies ScanModules still works (it uses a shared cache internally)
func TestCacheWithScanModules(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:terragrunt.hcl:" + Doc(
			Block("locals",
				Str("root", "value"),
			),
		).String(),
		"f:mod1/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("terraform",
				Str("source", "git::https://example.com/mod1.git"),
			),
		).String(),
		"f:mod2/terragrunt.hcl:" + Doc(
			Block("include",
				Labels("root"),
				Expr("path", `find_in_parent_folders()`),
			),
			Block("terraform",
				Str("source", "git::https://example.com/mod2.git"),
			),
		).String(),
	})

	// ScanModules should work and internally use a shared cache
	modules, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), false)
	if err != nil {
		t.Fatalf("ScanModules failed: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(modules))
	}

	// Verify both modules have correct sources
	sources := make(map[string]bool)
	for _, mod := range modules {
		sources[mod.Source] = true
	}

	expectedSources := []string{
		"git::https://example.com/mod1.git",
		"git::https://example.com/mod2.git",
	}

	for _, expected := range expectedSources {
		if !sources[expected] {
			t.Errorf("Expected module with source %q not found", expected)
		}
	}
}

// TestTransitiveDependencies verifies that caching correctly handles transitive dependencies.
// When a shared config file is included by multiple modules, each module should get all
// transitive dependencies discovered during parsing, even if the file is cached.
func TestTransitiveDependencies(t *testing.T) {
	t.Parallel()

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:common/shared.hcl:` + Doc(
			Expr("inputs",
				`{
  						data = jsondecode(read_tfvars_file("${get_repo_root()}/common/shared.tfvars"))
					}`),
		).String(),
		`f:common/shared.tfvars:`,
		`f:common/terragrunt.hcl:` + Doc(
			Block("terraform",
				Str("source", "https://some.etc/common"),
			),
			Block("locals",
				Expr("a", `read_terragrunt_config("shared.hcl")`),
			),
		).String(),
		`f:mod1/terragrunt.hcl:` + Doc(
			Block("terraform",
				Str("source", "https://some.etc/mod1"),
			),
			Block("locals",
				Expr("a", `read_terragrunt_config("${get_repo_root()}/common/shared.hcl")`),
			),
		).String(),
		`f:mod2/terragrunt.hcl:` + Doc(
			Block("terraform",
				Str("source", "https://some.etc/mod2"),
			),
			Block("locals",
				Expr("a", `read_terragrunt_config("${get_repo_root()}/common/shared.hcl")`),
			),
		).String(),
	})

	modules, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), true)
	if err != nil {
		t.Fatalf("ScanModules failed: %v", err)
	}

	if len(modules) != 3 {
		t.Fatalf("Expected 3 modules, got %d", len(modules))
	}

	// Verify each module has the correct dependencies
	for _, mod := range modules {
		switch mod.Path.String() {
		case "/common":
			if len(mod.DependsOn) != 2 {
				t.Errorf("Expected /common to have 2 dependencies, got %d: %v", len(mod.DependsOn), mod.DependsOn)
			}
			expectedDeps := map[string]bool{
				"/common/shared.hcl":    false,
				"/common/shared.tfvars": false,
			}
			for _, dep := range mod.DependsOn {
				if _, ok := expectedDeps[dep.String()]; ok {
					expectedDeps[dep.String()] = true
				}
			}
			for dep, found := range expectedDeps {
				if !found {
					t.Errorf("/common missing dependency: %s", dep)
				}
			}

		case "/mod1":
			if len(mod.DependsOn) != 2 {
				t.Errorf("Expected /mod1 to have 2 dependencies, got %d: %v", len(mod.DependsOn), mod.DependsOn)
			}
			expectedDeps := map[string]bool{
				"/common/shared.hcl":    false,
				"/common/shared.tfvars": false,
			}
			for _, dep := range mod.DependsOn {
				if _, ok := expectedDeps[dep.String()]; ok {
					expectedDeps[dep.String()] = true
				}
			}
			for dep, found := range expectedDeps {
				if !found {
					t.Errorf("/mod1 missing dependency: %s (this is the bug - transitive deps not cached)", dep)
				}
			}

		case "/mod2":
			if len(mod.DependsOn) != 2 {
				t.Errorf("Expected /mod2 to have 2 dependencies, got %d: %v", len(mod.DependsOn), mod.DependsOn)
			}
			expectedDeps := map[string]bool{
				"/common/shared.hcl":    false,
				"/common/shared.tfvars": false,
			}
			for _, dep := range mod.DependsOn {
				if _, ok := expectedDeps[dep.String()]; ok {
					expectedDeps[dep.String()] = true
				}
			}
			for dep, found := range expectedDeps {
				if !found {
					t.Errorf("/mod2 missing dependency: %s (this is the bug - transitive deps not cached)", dep)
				}
			}
		}
	}
}

// TestCacheDisabledViaEnvVar verifies that caching can be disabled via TM_DISABLE_TG_CACHE
func TestCacheDisabledViaEnvVar(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	// Set environment variable to disable caching
	t.Setenv("TM_DISABLE_TG_CACHE", "1")

	s := sandbox.New(t)
	s.BuildTree([]string{
		`f:common/shared.hcl:` + Doc(
			Expr("inputs",
				`{
  						data = "test"
					}`),
		).String(),
		`f:mod1/terragrunt.hcl:` + Doc(
			Block("terraform",
				Str("source", "https://some.etc/mod1"),
			),
			Block("locals",
				Expr("a", `read_terragrunt_config("${get_repo_root()}/common/shared.hcl")`),
			),
		).String(),
	})

	// ScanModules should work without caching
	modules, err := tg.ScanModules(s.RootDir(), project.NewPath("/"), true)
	if err != nil {
		t.Fatalf("ScanModules failed with caching disabled: %v", err)
	}

	if len(modules) != 1 {
		t.Fatalf("Expected 1 module, got %d", len(modules))
	}

	// Verify module has correct dependency even without caching
	mod := modules[0]
	if len(mod.DependsOn) != 1 {
		t.Errorf("Expected /mod1 to have 1 dependency, got %d: %v", len(mod.DependsOn), mod.DependsOn)
	}
	if mod.DependsOn[0].String() != "/common/shared.hcl" {
		t.Errorf("Expected dependency /common/shared.hcl, got %s", mod.DependsOn[0].String())
	}
}
