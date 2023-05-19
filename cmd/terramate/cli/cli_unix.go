// Copyright 2023 Mineiros GmbH
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

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package cli

import (
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
)

func userTerramateDir() (string, error) {
	homeDir, err := userHomeDir()
	if err != nil {
		return "", errors.E(err, "failed to discover the location of the local %s directory", terramateUserConfigDir)
	}
	return filepath.Join(homeDir, terramateUserConfigDir), nil
}
