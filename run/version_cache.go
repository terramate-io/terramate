// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import (
	"context"
	"os/exec"
	"regexp"
	"sync"
)

type versionEntry struct {
	once    sync.Once
	version string
}

var (
	versionCacheMu sync.Mutex
	versionCache   = map[string]*versionEntry{}
	testOverrideMu sync.Mutex
	testOverride   = map[string]string{}
)

func getVersionEntry(resolvedPath string) *versionEntry {
	versionCacheMu.Lock()
	defer versionCacheMu.Unlock()
	entry, ok := versionCache[resolvedPath]
	if !ok {
		entry = &versionEntry{}
		versionCache[resolvedPath] = entry
	}
	return entry
}

// ResolveVersion returns the semantic version of the given binary by executing
// "<binary> --version", parsed with a generic semver regex. The shell-out is
// performed at most once per resolved binary path across the process.
func ResolveVersion(ctx context.Context, environ []string, binary string) string {
	// test override by binary name
	testOverrideMu.Lock()
	if v, ok := testOverride[binary]; ok && v != "" {
		testOverrideMu.Unlock()
		return v
	}
	testOverrideMu.Unlock()

	cmdPath, err := LookPath(binary, environ)
	if err != nil {
		return ""
	}

	entry := getVersionEntry(cmdPath)
	entry.once.Do(func() {
		out, err := exec.CommandContext(ctx, cmdPath, "--version").CombinedOutput()
		if err != nil {
			return
		}
		re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			entry.version = matches[1]
		}
	})
	return entry.version
}

// ResolveVersionForResolvedPath returns the semantic version of the given binary
// at the provided resolved path by executing "<resolvedPath> --version".
// The shell-out is performed at most once per resolved binary path across the process.
func ResolveVersionForResolvedPath(ctx context.Context, resolvedPath string) string {
	entry := getVersionEntry(resolvedPath)
	entry.once.Do(func() {
		out, err := exec.CommandContext(ctx, resolvedPath, "--version").CombinedOutput()
		if err != nil {
			return
		}
		re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			entry.version = matches[1]
		}
	})
	return entry.version
}

// ResolveVersionFor returns the semantic version for a given binary. If a test
// override is configured for the binary name it is returned immediately. If a
// non-empty resolvedPath is provided, it will be used to avoid any additional
// path lookups. Otherwise it falls back to ResolveVersion which may look up the
// binary in PATH.
func ResolveVersionFor(ctx context.Context, environ []string, binary, resolvedPath string) string {
	// test override by binary name (keeps test behavior fast and deterministic)
	testOverrideMu.Lock()
	if v, ok := testOverride[binary]; ok && v != "" {
		testOverrideMu.Unlock()
		return v
	}
	testOverrideMu.Unlock()

	if resolvedPath != "" {
		return ResolveVersionForResolvedPath(ctx, resolvedPath)
	}
	return ResolveVersion(ctx, environ, binary)
}

// SeedVersionForResolvedPath sets the version for a given resolved binary path.
// Useful for tests to avoid shelling out.
func SeedVersionForResolvedPath(resolvedPath, version string) {
	entry := getVersionEntry(resolvedPath)
	entry.once.Do(func() { entry.version = version })
}

// ResetVersionCache clears the shared version cache. Intended for tests.
func ResetVersionCache() {
	versionCacheMu.Lock()
	defer versionCacheMu.Unlock()
	versionCache = map[string]*versionEntry{}
}

// SetTestVersionOverride sets a version override by binary name for tests.
// When set, ResolveVersion will return this value without shelling out.
func SetTestVersionOverride(binary, version string) {
	testOverrideMu.Lock()
	defer testOverrideMu.Unlock()
	if version == "" {
		delete(testOverride, binary)
		return
	}
	testOverride[binary] = version
}
