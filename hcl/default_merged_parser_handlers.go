// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// MergedBlockHandlerConstructor is a constructor for a merged block handler.
type MergedBlockHandlerConstructor func() MergedBlockHandler

// DefaultMergedBlockHandlers returns the default merged block specifications for the parser.
func DefaultMergedBlockHandlers() []MergedBlockHandlerConstructor {
	return []MergedBlockHandlerConstructor{
		newTerramateBlockConstructor,
	}
}

func newTerramateBlockConstructor() MergedBlockHandler {
	return NewTerramateBlockParser()
}
