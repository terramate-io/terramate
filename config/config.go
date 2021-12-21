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

package config

import (
	"os"
	"path/filepath"

	"github.com/mineiros-io/terramate/hcl"
)

const (
	// Filename is the name of the terramate configuration file.
	Filename = "terramate.tm.hcl"

	// DefaultInitConstraint is the default constraint used in stack initialization.
	DefaultInitConstraint = "~>"
)

// Exists tells if path has a terramate config file.
func Exists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !st.IsDir() {
		return false
	}

	fname := filepath.Join(path, Filename)
	info, err := os.Stat(fname)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular()
}

func TryLoadRootConfig(dir string) (found bool, cfg *hcl.Config, err error) {
	path := filepath.Join(dir, Filename)
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}

		return false, nil, err
	}

	cfg, err = hcl.ParseFile(path)
	if err != nil {
		return false, nil, err
	}
	return true, cfg, nil
}
