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
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultFilename is the name of the default Terramate configuration file.
	DefaultFilename = "terramate.tm.hcl"
)

const (
	// ErrSchema indicates that the configuration has an invalid schema.
	ErrSchema errors.Kind = "config has an invalid schema"
)

// TryLoadRootConfig try to load the Terramate root config. It looks for the
// the config in fromdir and all parent directories until / is reached.
// If the configuration is found, it returns configpath != "" and found as true.
func TryLoadRootConfig(fromdir string) (cfg hcl.Config, configpath string, found bool, err error) {
	for {
		logger := log.With().
			Str("action", "TryLoadRootConfig()").
			Str("path", fromdir).
			Logger()

		logger.Trace().Msg("Parse Terramate config.")

		cfg, err = hcl.ParseDir(fromdir, fromdir)
		if err != nil {
			// the imports only works for the correct rootdir.
			// As we are looking for the rootdir, we should ignore ErrImport
			// errors.
			if !errors.IsKind(err, hcl.ErrImport) {
				return hcl.Config{}, "", false, err
			}
		} else if cfg.Terramate != nil && cfg.Terramate.Config != nil {
			return cfg, fromdir, true, nil
		}

		parent, ok := parentDir(fromdir)
		if !ok {
			break
		}
		fromdir = parent
	}
	return hcl.Config{}, "", false, nil
}

// IsStack returns true if the given directory is a stack, false otherwise.
func IsStack(rootdir, dir string) (bool, error) {
	logger := log.With().
		Str("action", "config.IsStack()").
		Str("rootdir", rootdir).
		Str("dir", rootdir).
		Logger()

	cfg, err := hcl.ParseDir(rootdir, dir)
	if err != nil {
		logger.Trace().Err(err).Msg("unable to parse config, not a stack")
		return false, err
	}
	return cfg.Stack != nil, nil
}

func parentDir(dir string) (string, bool) {
	parent := filepath.Dir(dir)
	return parent, parent != dir
}
