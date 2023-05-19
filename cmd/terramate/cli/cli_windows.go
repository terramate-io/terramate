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

package cli

import (
	"github.com/mineiros-io/terramate/cmd/terramate/cli/cliconfig"
	"github.com/mineiros-io/terramate/errors"
	"os"
	"path/filepath"
)

func userTerramateDir() (string, error) {
	appdata := os.Getenv(cliconfig.DirEnv)
	if appdata == "" {
		return "", errors.E("APPDATA not set")
	}
	return filepath.Join(appdata, terramateUserConfigDir), nil
}
