// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package terragrunt

import (
	runpkg "github.com/terramate-io/terramate/run"
)

// SetRunnerVersionForTest sets a fake terragrunt version for tests.
// Only compiled in test builds.
func SetRunnerVersionForTest(_ *Runner, v string) {
	runpkg.SetTestVersionOverride("terragrunt", v)
}
