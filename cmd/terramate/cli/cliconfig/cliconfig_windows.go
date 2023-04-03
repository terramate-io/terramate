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

//go:build windows

package cliconfig

import (
	"os"
	"path/filepath"
)

// Filename is the name of the CLI configuration file.
const Filename = "terramate.rc"

// DirEnv is the environment variable used to define the config location.
const DirEnv = "APPDATA"

func configAbsPath() (string, bool) {
	appdata := os.Getenv(DirEnv)
	if appdata == "" {
		return "", false
	}
	return filepath.Join(appdata, Filename), true
}
