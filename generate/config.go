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
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/config"
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

func (s StackCfg) ExportedTfFilename(name string) string {
	// We may have customized configuration someday
	return fmt.Sprintf("_gen_terramate_%s.tf", name)
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

	logger.Trace().Msg("loading config.")

	parsedConfig, found, err := config.TryLoadRootConfig(configdir)
	if err != nil {
		return StackCfg{}, fmt.Errorf("loading stack config: %v", err)
	}

	if !found {
		logger.Trace().Msg("config file not found.")
		return loadStackCfg(root, filepath.Dir(configdir))
	}

	logger.Trace().Msg("loaded config.")

	if parsedConfig.Terramate.RootConfig.Generate == nil {
		logger.Trace().Msg("terramate.config.generate block not found.")
		return loadStackCfg(root, filepath.Dir(configdir))
	}

	logger.Trace().Msg("terramate.config.generate found.")

	gen := parsedConfig.Terramate.RootConfig.Generate

	return StackCfg{
		BackendCfgFilename: optstr(gen.BackendCfgFilename, BackendCfgFilename),
		LocalsFilename:     optstr(gen.LocalsFilename, LocalsFilename),
	}, nil
}

func optstr(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}
