// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import "github.com/hashicorp/go-version"

// IsCompatible reports if baseVersion satisfies the compatibleWith constraint.
// Empty compatibleWith values are treated as compatible.
func IsCompatible(baseVersion, compatibleWith string) (bool, error) {
	if compatibleWith == "" {
		return true, nil
	}
	v, err := version.NewVersion(baseVersion)
	if err != nil {
		return false, err
	}
	constraint, err := version.NewConstraint(compatibleWith)
	if err != nil {
		return false, err
	}
	return constraint.Check(v), nil
}
