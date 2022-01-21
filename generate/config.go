// Copyright 2022 Mineiros GmbH
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

package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

// StackCfg represents code generation configuration for a stack.
type StackCfg struct {
	BackendCfgFilename string
	LocalsFilename     string
}

const (
	// BackendCfgFilename is the name of the terramate generated tf file for backend configuration.
	BackendCfgFilename = "_gen_backend_cfg.tm.tf"

	// LocalsFilename is the name of the terramate generated tf file for exported locals.
	LocalsFilename = "_gen_locals.tm.tf"
)

// LoadStackCfg loads code generation configuration for the given stack.
func LoadStackCfg(root string, stack stack.S) (StackCfg, error) {
	return loadStackCfg(root, stack.AbsPath())
}

func loadStackCfg(root string, configdir string) (StackCfg, error) {
	logger := log.With().
		Str("action", "loadStackCfg()").
		Str("root", root).
		Str("configDir", configdir).
		Logger()

	logger.Trace().Msg("Check if still inside project root.")

	if !strings.HasPrefix(configdir, root) {
		logger.Trace().Msg("Outside project root, returning default config")
		return StackCfg{
			BackendCfgFilename: BackendCfgFilename,
			LocalsFilename:     LocalsFilename,
		}, nil
	}

	configfile := filepath.Join(configdir, config.Filename)
	logger = logger.With().
		Str("configFile", configfile).
		Logger()

	logger.Trace().Msg("checking for code generation config.")

	if _, err := os.Stat(configfile); err != nil {
		if os.IsNotExist(err) {
			logger.Trace().Msg("config file not found.")
			return loadStackCfg(root, filepath.Dir(configdir))
		}
		return StackCfg{}, fmt.Errorf("stat config file: %v", err)
	}

	logger.Trace().Msg("Read config file.")

	config, err := os.ReadFile(configfile)
	if err != nil {
		return StackCfg{}, fmt.Errorf("reading config: %v", err)
	}

	logger.Trace().Msg("Parse config file.")

	parsedConfig, err := hcl.Parse(configfile, config)
	if err != nil {
		return StackCfg{}, fmt.Errorf("parsing config: %w", err)
	}

	if parsedConfig.Terramate == nil {
		logger.Trace().Msg("terramate block not found.")
		return loadStackCfg(root, filepath.Dir(configdir))
	}

	terramate := parsedConfig.Terramate
	if terramate.RootConfig == nil {
		logger.Trace().Msg("terramate.config block not found.")
		return loadStackCfg(root, filepath.Dir(configdir))
	}

	tmconfig := terramate.RootConfig
	if tmconfig.Generate == nil {
		logger.Trace().Msg("terramate.config.generate block not found.")
		return loadStackCfg(root, filepath.Dir(configdir))
	}

	logger.Trace().Msg("terramate.config.generate found.")

	return StackCfg{
		BackendCfgFilename: tmconfig.Generate.BackendCfgFilename,
		LocalsFilename:     tmconfig.Generate.LocalsFilename,
	}, nil
}
