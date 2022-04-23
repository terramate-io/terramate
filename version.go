// Copyright 2021 Mineiros GmbH
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

package terramate

import (
	_ "embed"
	"strings"

	hclversion "github.com/hashicorp/go-version"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

//go:embed VERSION
var version string

// ErrVersion indicates failure when checking Terramate version.
const ErrVersion errors.Kind = "version check error"

// Version of terramate.
func Version() string {
	return strings.TrimSpace(version)
}

// CheckVersion checks Terramate version against the given constraint.
func CheckVersion(vconstraint string) error {
	version := Version()
	logger := log.With().
		Str("version", version).
		Str("constraint", vconstraint).
		Logger()

	logger.Trace().Msg("parsing version constraint")

	constraint, err := hclversion.NewConstraint(vconstraint)
	if err != nil {
		return errors.E(ErrVersion, "invalid constraint", err)
	}

	logger.Trace().Msg("parsing terramate version")

	semver, err := hclversion.NewSemver(version)
	if err != nil {
		return errors.E(ErrVersion, "terramate built with invalid version", err)
	}

	logger.Trace().Msg("checking version constraint")

	if !constraint.Check(semver) {
		return errors.E(
			ErrVersion,
			"version constraint %q not satisfied by terramate version %q",
			vconstraint,
			Version(),
		)
	}
	return nil
}
