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

package cliconfig

import (
	"os/user"
	"path/filepath"
)

// Filename is the name of the CLI configuration file.
const Filename = ".terramaterc"

// DirEnv is the environment variable used to define the config location.
const DirEnv = "HOME"

func configAbsPath() (string, bool) {
	usr, err := user.Current()
	if err != nil {
		return "", false
	}
	return filepath.Join(usr.HomeDir, Filename), true
}
