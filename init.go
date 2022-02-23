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

	"github.com/rs/zerolog/log"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
)

// Init initialize a stack. If the stack is already initialized it returns
// with no error and no changes will be performed on the stack.
func Init(root, dir string) error {
	logger := log.With().
		Str("action", "Init()").
		Str("stack", dir).
		Logger()

	logger.Trace().Msg("Check if directory is abs.")

	if !filepath.IsAbs(dir) {
		// TODO(i4k): this needs to go away soon.
		return errors.New("init requires an absolute path")
	}

	logger.Trace().Msg("Get directory info.")

	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("init requires an existing directory")
		}

		return fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	logger.Trace().Msg("Check if stack is leaf.")

	ok, err := stack.IsLeaf(root, dir)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("directory %q is not a leaf stack", dir)
	}

	logger.Trace().Msg("Lookup parent stack.")
	parentStack, found, err := stack.LookupParent(root, dir)
	if err != nil {
		return err
	}

	if found {
		return fmt.Errorf("directory %q is inside stack %q but nested stacks are disallowed",
			dir, parentStack.PrjAbsPath())
	}

	logger.Trace().Msg("Get stack info.")

	parsedCfg, err := hcl.ParseDir(dir)
	if err != nil {
		return fmt.Errorf("checking config for stack %q: %w", dir, err)
	}

	if parsedCfg.Stack != nil {
		logger.Debug().Msg("Stack already initialized, nothing to do")
		return nil
	}

	if !parsedCfg.IsEmpty() {
		return errors.New("dir has terramate config with no stack defined, expected empty or a stack")
	}

	logger.Debug().Msg("Create new configuration.")

	cfg, err := hcl.NewConfig(dir)
	if err != nil {
		return fmt.Errorf("failed to create new stack config: %w", err)
	}

	cfg.Stack = &hcl.Stack{
		Name: filepath.Base(dir),
	}

	logger.Debug().Msg("Save configuration.")

	err = cfg.Save(config.DefaultFilename)
	if err != nil {
		return fmt.Errorf(
			"failed to write %q on stack %q: %w",
			config.DefaultFilename,
			dir,
			err,
		)
	}

	return nil
}
