// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// UniqueBlockHandlerConstructor is a constructor for a unique block handler.
type UniqueBlockHandlerConstructor func() UniqueBlockHandler

// DefaultUniqueBlockHandlers returns the default unique block specifications for the parser.
func DefaultUniqueBlockHandlers() []UniqueBlockHandlerConstructor {
	return []UniqueBlockHandlerConstructor{
		newVendorBlockConstructor,
	}
}

func newVendorBlockConstructor() UniqueBlockHandler {
	return NewVendorBlockParser()
}
