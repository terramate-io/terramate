// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// MergedLabelsBlockHandlerConstructor is a constructor for a merged labels block handler.
type MergedLabelsBlockHandlerConstructor func() MergedLabelsBlockHandler

// DefaultMergedLabelsBlockHandlers returns the default merged block specifications for the parser.
func DefaultMergedLabelsBlockHandlers() []MergedLabelsBlockHandlerConstructor {
	return []MergedLabelsBlockHandlerConstructor{
		newGlobalsBlockConstructor,
	}
}

func newGlobalsBlockConstructor() MergedLabelsBlockHandler {
	return NewGlobalsBlockParser()
}
