// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// DefaultUnmergedBlockParsers returns the default unmerged block specifications for the parser.
func DefaultUnmergedBlockParsers() []UnmergedBlockHandler {
	return []UnmergedBlockHandler{
		NewStackBlockParser(),
		NewAssertBlockParser(nil),
		NewGenerateHCLBlockParser(),
		NewGenerateFileBlockParser(),
		NewSharingBackendBlockParser(),
		NewInputBlockParser(),
		NewOutputBlockParser(),
		NewScriptBlockParser(),
	}
}

// DefaultMergedBlockHandlers returns the default merged block specifications for the parser.
func DefaultMergedBlockHandlers() []MergedBlockHandler {
	return []MergedBlockHandler{
		NewTerramateBlockParser(),
	}
}

// DefaultMergedLabelsBlockHandlers returns the default merged block specifications for the parser.
func DefaultMergedLabelsBlockHandlers() []MergedLabelsBlockHandler {
	return []MergedLabelsBlockHandler{
		NewGlobalsBlockParser(),
	}
}

// DefaultUniqueBlockHandlers returns the default unique block specifications for the parser.
func DefaultUniqueBlockHandlers() []UniqueBlockHandler {
	return []UniqueBlockHandler{
		NewVendorBlockParser(),
	}
}
