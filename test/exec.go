// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package test

import (
	"os/exec"
	"testing"
)

// LookPath is identical to exec.LookPath except it will
// fail the test if the lookup fails
func LookPath(t *testing.T, file string) string {
	t.Helper()

	path, err := exec.LookPath(file)
	if err != nil {
		t.Fatalf("exec.LookPath(%q) = %v", file, err)
	}
	return path
}
