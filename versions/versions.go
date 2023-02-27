// Copyright 2023 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package versions

import (
	"github.com/apparentlymart/go-versions/versions"
	"github.com/apparentlymart/go-versions/versions/constraints"
	hclversion "github.com/hashicorp/go-version"
	"github.com/mineiros-io/terramate/errors"
)

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
	var check func() bool

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
		check = func() bool {
			return allowed.Has(semver)
		}
	} else {
		spec, err := hclversion.NewConstraint(constraint)
		if err != nil {
			return false, errors.E(ErrCheck, err, "invalid constraint")
		}

		semver, err := hclversion.NewSemver(version)
		if err != nil {
			return false, errors.E(ErrCheck, err, "invalid version")
		}

		check = func() bool {
			return spec.Check(semver)
		}
	}

	return check(), nil
}
