// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/event"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/project"
)

// API for code generation.
type API interface {
	// Do runs the code generation.
	Do(
		root *config.Root,
		targetDir project.Path,
		parallel int,
		vendorDir project.Path,
		vendorRequests chan<- event.VendorRequest,
	) *genreport.Report

	// DetectOutdated checks for outdated files that would be changed by Do, but without making any changes.
	DetectOutdated(
		root *config.Root,
		target *config.Tree,
		vendorDir project.Path,
	) ([]string, error)

	// Load will load return all the generated files within a project, but without making any changes.
	Load(
		root *config.Root,
		vendorDir project.Path,
	) ([]LoadResult, error)
}
