//go:build !windows

// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tofu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runpkg "github.com/terramate-io/terramate/run"
)

func TestTofuVersionIsCachedPerResolvedPath(t *testing.T) {
	// reset cache for test isolation
	runpkg.ResetVersionCache()

	tdir := t.TempDir()
	counter := filepath.Join(tdir, "count.txt")
	script := filepath.Join(tdir, "tofu")

	scriptContent := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		"  echo \"OpenTofu v1.7.0\"\n" +
		"fi\n" +
		"echo x >> '" + strings.ReplaceAll(counter, "'", "'\\''") + "'\n"
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}

	env := []string{"PATH=" + tdir}
	r1 := NewRunner(env, tdir)
	r2 := NewRunner(env, tdir)
	ctx := context.Background()

	if v := runpkg.ResolveVersion(ctx, r1.Env, "tofu"); v == "" {
		t.Fatalf("expected version, got empty")
	}
	_ = runpkg.ResolveVersion(ctx, r1.Env, "tofu")
	_ = runpkg.ResolveVersion(ctx, r2.Env, "tofu")

	b, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if got := strings.Count(string(b), "x"); got != 1 {
		t.Fatalf("expected one invocation, got %d", got)
	}
}
