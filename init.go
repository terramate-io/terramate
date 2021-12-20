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

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/stack"
)

// Init initialize a stack. It's an error to initialize an already initialized
// stack unless they are of same versions. In case the stack is initialized with
// other terramate version, the force flag can be used to explicitly initialize
// it anyway. The dir must be an absolute path.
func Init(dir string, force bool) error {
	if !filepath.IsAbs(dir) {
		// TODO(i4k): this needs to go away soon.
		return errors.New("init requires an absolute path")
	}
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("init requires an existing directory")
		}

		return fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	stackfile := filepath.Join(dir, config.Filename)
	isInitialized := false

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
		vconstraint, err := parseVersion(stackfile)
		if err != nil {
			return fmt.Errorf("stack already initialized: error fetching "+
				"version: %w", err)
		}

		constraint, err := hclversion.NewConstraint(vconstraint)
		if err != nil {
			return fmt.Errorf("unable to check stack constraint: %w", err)
		}

		if !constraint.Check(tfversionObj) {
			return fmt.Errorf("stack version constraint %q do not match terramate "+
				"version %q", vconstraint, Version())
		}
	}

	ok, err := stack.IsLeaf(dir)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("directory %q is not a leaf stack", dir)
	}

	parentStack, found, err := stack.LookupParent(dir)
	if err != nil {
		return err
	}

	if found {
		return fmt.Errorf("directory %q is inside stack %q but nested stacks are disallowed",
			dir, parentStack.Dir)
	}

	f, err := os.Create(stackfile)
	if err != nil {
		return err
	}

	defer f.Close()

	cfg := hcl.NewConfig(DefaultVersionConstraint())
	cfg.Stack = &hcl.Stack{
		Name: filepath.Base(dir),
	}

	err = hcl.PrintConfig(f, cfg)
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
	config, err := hcl.ParseFile(stackfile)
	if err != nil {
		return "", fmt.Errorf("failed to parse file %q: %w", stackfile, err)
	}

	return config.Terramate.RequiredVersion, nil
}
