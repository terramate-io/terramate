// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package runner

import "path/filepath"

// TerramatePath returns the path to the test-built terramate binary.
func TerramatePath() string {
	if toolsetTestPath == "" {
		panic("runner is not initialized: use runner.Setup()")
	}
	return filepath.Join(toolsetTestPath, "terramate") + platExeSuffix()
}
