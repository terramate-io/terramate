// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stack

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
)

const (
	// ErrInvalidStackDir indicates that the given stack dir is invalid.
	ErrInvalidStackDir errors.Kind = "invalid stack directory"

	// ErrStackAlreadyExists indicates that the stack already exists and cant be created.
	ErrStackAlreadyExists errors.Kind = "stack already exists"

	// ErrStackDefaultCfgFound indicates that the dir already has a default
	// stack configuration.
	ErrStackDefaultCfgFound errors.Kind = "default configuration file for stack already exists"
)

// DefaultFilename is the default file name for created stacks.
const DefaultFilename = "stack.tm.hcl"

const (
	createDirMode = 0755
)

// Create creates the provided stack on the filesystem.
// The list of import paths provided are generated inside the stack file.
//
// If the stack already exists it will return an error and no changes will be
// made to the stack.
func Create(root *config.Root, stack config.Stack, imports ...string) (err error) {
	logger := log.With().
		Str("action", "stack.Create()").
		Logger()

	err = stack.Validate()
	if err != nil {
		return err
	}

	if strings.HasPrefix(path.Base(stack.Dir.String()), ".") {
		return errors.E(ErrInvalidStackDir, "dot directories not allowed")
	}

	targetNode, ok := root.Lookup(stack.Dir)
	if ok && targetNode.IsStack() {
		return errors.E(ErrStackAlreadyExists)
	}

	logger.Trace().Msg("creating stack dir if absent")

	hostpath := stack.Dir.HostPath(root.HostDir())
	if err := os.MkdirAll(hostpath, createDirMode); err != nil {
		return errors.E(err, "failed to create new stack directories")
	}

	logger.Trace().Msg("validating create configuration")

	_, err = os.Stat(filepath.Join(hostpath, DefaultFilename))
	if err == nil {
		// Even if there is no stack block inside the file, we can't overwrite
		// the user file anyway.
		return errors.E(ErrStackDefaultCfgFound)
	}

	stackCfg := hcl.Stack{
		ID:          stack.ID,
		Name:        stack.Name,
		Description: stack.Description,
		After:       stack.After,
		Before:      stack.Before,
		Tags:        stack.Tags,
	}

	tmCfg, err := hcl.NewConfig(hostpath)
	if err != nil {
		return errors.E(err, "failed to create new stack config")
	}

	tmCfg.Stack = &stackCfg

	logger.Trace().Msg("creating stack file")

	stackFile, err := os.Create(filepath.Join(hostpath, DefaultFilename))
	if err != nil {
		return errors.E(err, "creating/truncating stack file")
	}

	defer func() {
		errClose := stackFile.Close()
		if errClose != nil {
			err = errors.L(err, errClose)
		}
	}()

	if err := hcl.PrintConfig(stackFile, tmCfg); err != nil {
		return errors.E(err, "writing stack config to stack file")
	}

	if len(imports) > 0 {
		fmt.Fprint(stackFile, "\n")
	}

	if err := hcl.PrintImports(stackFile, imports); err != nil {
		return errors.E(err, "writing stack imports to stack file")
	}

	return nil
}
