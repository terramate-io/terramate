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
	"fmt"
	"strings"

	tfversion "github.com/hashicorp/go-version"
	"github.com/rs/zerolog/log"
)

//go:embed VERSION
var version string

var tfversionObj *tfversion.Version

func init() {
	logger := log.With().
		Str("action", "init()").
		Str("version", Version()).
		Logger()

	logger.Debug().
		Msg("Parsing version as semver.")
	var err error
	tfversionObj, err = tfversion.NewSemver(Version())
	if err != nil {
		msg := fmt.Sprintf(
			"terramate version does not adhere to semver specification: %s",
			err.Error(),
		)
		panic(msg)
	}
}

// Version of terramate.
func Version() string {
	return strings.TrimSpace(version)
}
