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

package config

import (
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultFilename is the name of the default Terramate configuration file.
	DefaultFilename = "terramate.tm.hcl"

	// DefaultInitConstraint is the default constraint used in stack initialization.
	DefaultInitConstraint = "~>"
)

func TryLoadRootConfig(dir string) (cfg hcl.Config, found bool, err error) {
	logger := log.With().
		Str("action", "TryLoadRootConfig()").
		Str("path", dir).
		Logger()

	logger.Trace().Msg("Parse Terramate config.")

	cfg, err = hcl.ParseDir(dir)
	if err != nil {
		return hcl.Config{}, false, err
	}

	if cfg.Terramate != nil && cfg.Terramate.RootConfig != nil {
		return cfg, true, nil
	}
	return hcl.Config{}, false, nil
}
