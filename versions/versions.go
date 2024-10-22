// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package versions

import (
	"github.com/apparentlymart/go-versions/versions"
	"github.com/apparentlymart/go-versions/versions/constraints"
	hclversion "github.com/hashicorp/go-version"
	"github.com/terramate-io/terramate/errors"
)

// ErrCheck indicates the version doesn't match the constraint for any reason.
const ErrCheck errors.Kind = "version check error"

// Check checks if version matches the provided constraint and fails otherwise.
// For just checking if they match, use the [Match] function.
func Check(version string, constraint string, allowPrereleases bool) error {
	match, err := Match(version, constraint, allowPrereleases)
	if err != nil {
		return err
	}

	if !match {
		return errors.E(
			ErrCheck,
			"version constraint %q not satisfied by terramate version %q",
			constraint,
			version,
		)
	}
	return nil
}

// Match checks if version matches the given constraint.
// It only returns an error in the case of invalid version or constraint string.
func Match(version, constraint string, allowPrereleases bool) (bool, error) {
	if allowPrereleases {
		semver, err := versions.ParseVersion(version)
		if err != nil {
			return false, errors.E(ErrCheck, "terramate built with invalid version", err)
		}

		spec, err := constraints.ParseRubyStyleMulti(constraint)
		if err != nil {
			return false, errors.E(ErrCheck, "invalid constraint", err)
		}

		allowed := versions.MeetingConstraintsExact(spec)
		return allowed.Has(semver), nil
	}

	spec, err := hclversion.NewConstraint(constraint)
	if err != nil {
		return false, errors.E(ErrCheck, err, "invalid constraint")
	}
	semver, err := hclversion.NewSemver(version)
	if err != nil {
		return false, errors.E(ErrCheck, err, "invalid version")
	}
	return spec.Check(semver), nil
}
