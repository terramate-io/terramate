// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package manifest provides types and functions for loading package manifests.
package manifest

import (
	"encoding/json"
	"os"
)

// Package of bundles and components that can be found at a common location.
type Package struct {
	// Name of the package.
	Name string `json:"name"`

	// Description of the package.
	Description string `json:"description,omitempty"`

	// Location can be used to set an external location for bundles and and components of this entry.
	// If omitted, the location of the catalog file is used.
	Location string `json:"location,omitempty"`

	// Bundles described by this entry.
	Bundles []Bundle `json:"bundles,omitempty"`

	// Components described by this entry.
	Components []Component `json:"components,omitempty"`
}

// Bundle description contained within a manifest.
type Bundle struct {
	// Path to the bundle, relative to the package location.
	Path string `json:"path"`

	// Name of the bundle.
	Name string `json:"name"`

	// Class of the bundle.
	Class string `json:"class"`

	// Version of the bundle.
	Version string `json:"version"`

	// Description of the bundle.
	Description string `json:"description,omitempty"`

	// Technologies related to the bundle.
	Technologies []string `json:"technologies,omitempty"`
}

// Component description contained within a manifest.
type Component struct {
	// Path to the component, relative to the package location.
	Path string `json:"path"`

	// Name of the component.
	Name string `json:"name"`

	// Class of the component.
	Class string `json:"class"`

	// Version of the component.
	Version string `json:"version"`

	// Description of the component.
	Description string `json:"description,omitempty"`

	// Technologies related to the component.
	Technologies []string `json:"technologies,omitempty"`
}

// LoadFile reads and parses a JSON manifest file at the given path.
func LoadFile(path string) ([]*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkgs []*Package
	if err := json.Unmarshal(data, &pkgs); err != nil {
		return nil, err
	}

	return pkgs, nil
}
