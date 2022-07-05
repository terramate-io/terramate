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

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog/log"
)

const (
	// ErrInvalidStackDir indicates that the given stack dir is invalid
	ErrInvalidStackDir errors.Kind = "invalid stack directory"
)

// CreateCfg represents stack creation configuration.
type CreateCfg struct {
	// Dir is the relative path of the directory, inside the project root,
	// where the stack will be created. It must be non-empty.
	Dir string

	// ID of the stack, defaults to an UUID V4 if empty.
	ID string

	// Name of the stack, defaults to Dir basename if empty.
	Name string

	// Description of the stack, defaults to Name if empty.
	Description string

	// Imports represents a set of import paths.
	Imports []string
}

// Create creates a stack according to the given configuration.
// Any dirs on the path provided on the configuration that doesn't exist
// will be created.
//
// If the stack already exists it will return with no error and no changes
// will be performed on the stack.
func Create(rootdir string, cfg CreateCfg) error {
	const stackFilename = "stack.tm.hcl"

	logger := log.With().
		Str("action", "stack.Create()").
		Stringer("cfg", cfg).
		Logger()

	logger.Trace().Msg("validating create configuration")

	if strings.HasPrefix(filepath.Base(cfg.Dir), ".") {
		return errors.E(ErrInvalidStackDir, "dot directories not allowed")
	}

	logger.Trace().Msg("creating stack dir if absent")

	absdir := filepath.Join(rootdir, cfg.Dir)
	err := os.MkdirAll(absdir, 0755)
	if err != nil {
		return errors.E(err, "failed to create new stack directories")
	}

	hclCfg, err := hcl.NewConfig(absdir)
	if err != nil {
		return errors.E(err, "failed to create new stack config")
	}

	if cfg.ID == "" {
		logger.Trace().Msg("no ID provided, generating one")

		id, err := uuid.NewRandom()
		if err != nil {
			return errors.E(err, "failed to create stack UUID")
		}
		cfg.ID = id.String()
	}

	if cfg.Name == "" {
		cfg.Name = filepath.Base(cfg.Dir)
	}

	if cfg.Description == "" {
		cfg.Description = cfg.Name
	}

	hclID, err := hcl.NewStackID(cfg.ID)
	if err != nil {
		return errors.E(err, "new stack ID is invalid")
	}

	hclCfg.Stack = &hcl.Stack{
		ID:          hclID,
		Name:        cfg.Name,
		Description: cfg.Description,
	}

	logger.Trace().Msg("creating stack file")

	stackFile, err := os.Create(filepath.Join(absdir, stackFilename))
	if err != nil {
		return errors.E(err, "opening stack file")
	}
	defer func() {
		err := stackFile.Close()
		if err != nil {
			logger.Error().Err(err).Msg("closing stack file")
		}
	}()

	if err := hcl.PrintConfig(stackFile, hclCfg); err != nil {
		return errors.E(err, "writing stack config to stack file")
	}

	return hcl.PrintImports(stackFile, cfg.Imports)
}

func (cfg CreateCfg) String() string {
	return fmt.Sprintf("dir:%s, name:%s, desc:%s, imports:%v",
		cfg.Dir, cfg.Name, cfg.Description, cfg.Imports)
}
