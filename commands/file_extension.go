// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package commands

import "strings"

// FixupFileExtension adjusts extension according to output format.
func FixupFileExtension(format, fn string) string {
	switch format {
	case "yaml":
		if strings.HasSuffix(fn, ".hcl") {
			return strings.TrimSuffix(fn, "hcl") + "yml"
		}
		if strings.HasSuffix(fn, ".tm") {
			return fn + ".yml"
		}
	case "hcl":
		if strings.HasSuffix(fn, ".yml") {
			return strings.TrimSuffix(fn, "yml") + "hcl"
		}
		if strings.HasSuffix(fn, ".yaml") {
			return strings.TrimSuffix(fn, "yaml") + "hcl"
		}
		if strings.HasSuffix(fn, ".tm") {
			return fn + ".hcl"
		}
	}
	return fn
}
