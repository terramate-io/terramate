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

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
)

const (
	// ErrInvalidStackDir indicates that the given stack dir is invalid.
	ErrInvalidStackDir errors.Kind = "invalid stack directory"

	// ErrInvalidStackID indicates that the given stack ID is invalid.
	ErrInvalidStackID errors.Kind = "invalid stack ID"

	// ErrStackAlreadyExists indicates that the stack already exists and cant be created.
	ErrStackAlreadyExists errors.Kind = "stack already exists"

	// ErrStackDefaultCfgFound indicates that the dir already has a default
	// stack configuration.
	ErrStackDefaultCfgFound errors.Kind = "default configuration file for stack already exists"
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

	// After is the set of after stacks.
	After []string

	// Before is the set of before stacks.
	Before []string
}

const (
	createDirMode = 0755
)

// Create creates a stack according to the given configuration.
// Any dirs on the path provided on the configuration that doesn't exist
// will be created.
//
// If the stack already exists it will return an error and no changes will be
// made to the stack.
func Create(root *config.Root, cfg CreateCfg) (err error) {
	logger := log.With().
		Str("action", "stack.Create()").
		Stringer("cfg", cfg).
		Logger()

	rootdir := root.Dir()
	if !strings.HasPrefix(cfg.Dir, rootdir) {
		return errors.E(ErrInvalidStackDir, "stack %q must be inside project root %q", cfg.Dir, rootdir)
	}

	if strings.HasPrefix(filepath.Base(cfg.Dir), ".") {
		return errors.E(ErrInvalidStackDir, "dot directories not allowed")
	}

	logger.Trace().Msg("creating stack dir if absent")

	if err := os.MkdirAll(cfg.Dir, createDirMode); err != nil {
		return errors.E(err, "failed to create new stack directories")
	}

	logger.Trace().Msg("validating create configuration")

	_, err = os.Stat(filepath.Join(cfg.Dir, DefaultFilename))
	if err == nil {
		// Even if there is no stack inside the file, we can't overwrite
		// the user file anyway.
		return errors.E(ErrStackDefaultCfgFound)
	}

	// We could have a stack definition somewhere else.
	targetNode, ok := root.Lookup(project.PrjAbsPath(rootdir, cfg.Dir))
	if ok && targetNode.IsStack() {
		return errors.E(ErrStackAlreadyExists)
	}

	stackCfg := hcl.Stack{
		After:  cfg.After,
		Before: cfg.Before,
	}

	if cfg.Name != "" {
		stackCfg.Name = cfg.Name
	}

	if cfg.Description != "" {
		stackCfg.Description = cfg.Description
	}

	if cfg.ID != "" {
		stackID, err := hcl.NewStackID(cfg.ID)
		if err != nil {
			return errors.E(ErrInvalidStackID, err)
		}
		stackCfg.ID = stackID
	}

	tmCfg, err := hcl.NewConfig(cfg.Dir)
	if err != nil {
		return errors.E(err, "failed to create new stack config")
	}

	tmCfg.Stack = &stackCfg

	logger.Trace().Msg("creating stack file")

	err = func() error {
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
			return errors.E(err, "writing stack config to stack file")
		}

		if len(cfg.Imports) > 0 {
			fmt.Fprint(stackFile, "\n")
		}

		if err := hcl.PrintImports(stackFile, cfg.Imports); err != nil {
			return errors.E(err, "writing stack imports to stack file")
		}
		return nil
	}()

	if err != nil {
		return err
	}

	return root.LoadSubTree(project.PrjAbsPath(rootdir, cfg.Dir))
}

func (cfg CreateCfg) String() string {
	return fmt.Sprintf("dir:%s, name:%s, desc:%s, imports:%v",
		cfg.Dir, cfg.Name, cfg.Description, cfg.Imports)
}
