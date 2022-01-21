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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	hclversion "github.com/hashicorp/go-version"
	"github.com/rs/zerolog/log"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
)

// Init initialize a stack. It's an error to initialize an already initialized
// stack unless they are of same versions. In case the stack is initialized with
// other terramate version, the force flag can be used to explicitly initialize
// it anyway. The dir must be an absolute path.
func Init(root, dir string, force bool) error {
	logger := log.With().
		Str("action", "Init()").
		Str("stack", dir).
		Logger()

	logger.Trace().
		Msg("Check if directory is abs.")
	if !filepath.IsAbs(dir) {
		// TODO(i4k): this needs to go away soon.
		return errors.New("init requires an absolute path")
	}

	logger.Trace().
		Msg("Get directory info.")
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("init requires an existing directory")
		}

		return fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	logger.Trace().
		Msg("Check if stack is leaf.")
	ok, err := stack.IsLeaf(root, dir)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("directory %q is not a leaf stack", dir)
	}

	logger.Trace().
		Msg("Lookup parent stack.")
	parentStack, found, err := stack.LookupParent(root, dir)
	if err != nil {
		return err
	}

	if found {
		return fmt.Errorf("directory %q is inside stack %q but nested stacks are disallowed",
			dir, parentStack.Dir)
	}

	logger.Trace().Msg("Get stack file.")

	stackfile := filepath.Join(dir, config.Filename)
	isInitialized := false

	logger = log.With().
		Str("action", "Init()").
		Str("stack", dir).
		Str("configFile", stackfile).
		Logger()

	logger.Trace().Msg("Get stack file info.")

	st, err := os.Stat(stackfile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat failed on %q: %w", stackfile, err)
		}
	} else {
		isInitialized = true
	}

	if isInitialized && !st.Mode().IsRegular() {
		return fmt.Errorf("the path %q is not a regular file", stackfile)
	}

	if isInitialized && !force {
		logger.Trace().Msg("Stack is initialized and not forced.")

		logger.Trace().Msg("Parse version.")

		vconstraint, err := parseVersion(stackfile)
		if err != nil {
			return fmt.Errorf("stack already initialized: error fetching "+
				"version: %w", err)
		}

		logger.Trace().Msg("Create new constraint from version.")

		constraint, err := hclversion.NewConstraint(vconstraint)
		if err != nil {
			return fmt.Errorf("unable to check stack constraint: %w", err)
		}

		if !constraint.Check(tfversionObj) {
			return fmt.Errorf("stack version constraint %q do not match terramate "+
				"version %q", vconstraint, Version())
		}
	}

	logger.Debug().Msg("Create new configuration.")

	cfg, err := hcl.NewConfig(dir, DefaultVersionConstraint())
	if err != nil {
		return fmt.Errorf("failed to create new stack config: %w", err)
	}

	cfg.Stack = &hcl.Stack{
		Name: filepath.Base(dir),
	}

	logger.Debug().Msg("Save configuration.")

	err = cfg.Save(config.Filename)
	if err != nil {
		return fmt.Errorf("failed to write %q: %w", stackfile, err)
	}

	return nil
}

// DefaultVersionConstraint is the default version constraint used by terramate
// when generating tm files.
func DefaultVersionConstraint() string {
	return config.DefaultInitConstraint + " " + Version()
}

func parseVersion(stackfile string) (string, error) {
	logger := log.With().
		Str("action", "parseVersion()").
		Str("configFile", stackfile).
		Logger()

	logger.Debug().
		Msg("Parse stack file.")
	config, err := hcl.ParseFile(stackfile)
	if err != nil {
		return "", fmt.Errorf("failed to parse file %q: %w", stackfile, err)
	}

	return config.Terramate.RequiredVersion, nil
}
