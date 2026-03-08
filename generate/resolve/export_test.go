// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve

// NewResolverForTest creates a Resolver with the given cache directory for testing.
func NewResolverForTest(cachedir string) *Resolver {
	return &Resolver{
		cachedir: cachedir,
	}
}
