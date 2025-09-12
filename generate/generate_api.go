// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/project"
)

// API for code generation.
// Currently, it only contains DetectOutdated, but it will be extended in the future.
type API interface {
	DetectOutdated(root *config.Root, target *config.Tree, vendorDir project.Path) ([]string, error)
}
