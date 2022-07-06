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
	// ErrInvalidStackDir indicates that the given stack dir is invalid.
	ErrInvalidStackDir errors.Kind = "invalid stack directory"

	// ErrInvalidStackID indicates that the given stack ID is invalid.
	ErrInvalidStackID errors.Kind = "invalid stack ID"

	// ErrStackAlreadyExists indicates that the stack already exists and cant be created.
	ErrStackAlreadyExists errors.Kind = "stack already exists"
)

// DefaultFilename is the default file name for created stacks.
const DefaultFilename = "stack.tm.hcl"

// CreateCfg represents stack creation configuration.
type CreateCfg struct {
	// Dir is the absolute path of the directory, inside the project root,
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
// If the stack already exists it will return an error and no changes will be
// made to the stack.
func Create(rootdir string, cfg CreateCfg) (err error) {
	logger := log.With().
		Str("action", "stack.Create()").
		Stringer("cfg", cfg).
		Logger()

	if !filepath.IsAbs(cfg.Dir) {
		return errors.E(ErrInvalidStackDir, "relative paths not allowed")
	}

	if !strings.HasPrefix(cfg.Dir, rootdir) {
		return errors.E(ErrInvalidStackDir, "stack %q must be inside project root %q", cfg.Dir, rootdir)
	}

	if strings.HasPrefix(filepath.Base(cfg.Dir), ".") {
		return errors.E(ErrInvalidStackDir, "dot directories not allowed")
	}

	logger.Trace().Msg("creating stack dir if absent")

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return errors.E(err, "failed to create new stack directories")
	}

	logger.Trace().Msg("validating create configuration")

	_, err = os.Stat(filepath.Join(cfg.Dir, DefaultFilename))
	if err == nil {
		// Even if there is no stack inside the file, we can't overwrite
		// the user file anyway.
		return errors.E(ErrStackAlreadyExists, "%q already exists", DefaultFilename)
	}

	// We could have a stack definition somewhere else.
	parsedCfg, err := hcl.ParseDir(rootdir, cfg.Dir)
	if err != nil {
		return errors.E(err, "invalid config creating stack dir %s", cfg.Dir)
	}

	if parsedCfg.Stack != nil {
		return errors.E(ErrStackAlreadyExists, "name %q", parsedCfg.Stack.Name)
	}

	tmCfg, err := hcl.NewConfig(cfg.Dir)
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

	stackID, err := hcl.NewStackID(cfg.ID)
	if err != nil {
		return errors.E(ErrInvalidStackID, err)
	}

	tmCfg.Stack = &hcl.Stack{
		ID:          stackID,
		Name:        cfg.Name,
		Description: cfg.Description,
	}

	logger.Trace().Msg("creating stack file")

	stackFile, err := os.Create(filepath.Join(cfg.Dir, DefaultFilename))
	if err != nil {
		return errors.E(err, "opening stack file")
	}

	defer func() {
		errClose := stackFile.Close()
		if errClose != nil {
			if err != nil {
				err = errors.L(err, errClose)
			} else {
				err = errClose
			}
		}
	}()

	if err := hcl.PrintConfig(stackFile, tmCfg); err != nil {
		return errors.E(err, "writing stack imports to stack file")
	}

	if len(cfg.Imports) > 0 {
		fmt.Fprint(stackFile, "\n")
	}

	return hcl.PrintImports(stackFile, cfg.Imports)
}

func (cfg CreateCfg) String() string {
	return fmt.Sprintf("dir:%s, name:%s, desc:%s, imports:%v",
		cfg.Dir, cfg.Name, cfg.Description, cfg.Imports)
}
