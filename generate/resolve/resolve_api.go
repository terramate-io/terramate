// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package resolve is responsible for resolving and fetching sources for package items.
package resolve

import (
	"github.com/terramate-io/terramate/project"
)

// Kind represents the type of source being resolved.
type Kind int

const (
	// Bundle represents a bundle source kind.
	Bundle Kind = iota
	// Component represents a component source kind.
	Component
	// Schema represents a schema source kind.
	Schema
	// Manifest represents a manifest source kind.
	Manifest
)

// API defines an API for resolving source addresses.
type API interface {
	Resolve(rootdir string, src string, kind Kind, allowFetch bool, opts ...Option) (project.Path, error)
}

// Option is a functional option for configuring resolution behavior.
type Option func(API, *OptionValues)

// WithParentSource returns an Option that sets the parent source for relative path resolution.
func WithParentSource(parentSrc string) Option {
	return func(_ API, optData *OptionValues) {
		optData.ParentSrc = parentSrc
	}
}

// OptionValues holds the option values for source resolution.
type OptionValues struct {
	ParentSrc string
}
