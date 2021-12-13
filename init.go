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

	"github.com/mineiros-io/terramate/hcl"
)

const (
	// ConfigFilename is the name of the terramate configuration file.
	ConfigFilename = "terramate.tm.hcl"

	// DefaultInitConstraint is the default constraint used in stack initialization.
	DefaultInitConstraint = "~>"
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
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("init requires an existing directory")
		}

		return fmt.Errorf("stat failed on %q: %w", dir, err)
	}

	if !st.IsDir() {
		return errors.New("path is not a directory")
	}

	stackfile := filepath.Join(dir, ConfigFilename)
	isInitialized := false

	st, err = os.Stat(stackfile)
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

		err = os.Remove(string(stackfile))
		if err != nil {
			return fmt.Errorf("while removing %q: %w", stackfile, err)
		}
	}

	f, err := os.Create(stackfile)
	if err != nil {
		return err
	}

	defer f.Close()

	var p hcl.Printer
	err = p.PrintTerramate(f, hcl.Terramate{
		RequiredVersion: DefaultVersionConstraint(),
	})

	if err != nil {
		return fmt.Errorf("failed to write %q: %w", stackfile, err)
	}

	return nil
}

// DefaultVersionConstraint is the default version constraint used by terramate
// when generating tm files.
func DefaultVersionConstraint() string {
	return DefaultInitConstraint + " " + Version()
}

func parseVersion(stackfile string) (string, error) {
	parser := hcl.NewParser()
	ts, err := parser.ParseFile(stackfile)
	if err != nil {
		return "", fmt.Errorf("failed to parse file %q: %w", stackfile, err)
	}

	return ts.RequiredVersion, nil
}
