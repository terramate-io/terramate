// Copyright 2022 Mineiros GmbH
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

//go:build linux

package modvendor

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
)

func targetPathDir(vendorDir project.Path, modsrc tf.Source) project.Path {
	return project.NewPath(
		path.Join(vendorDir.String(), modsrc.Path, modsrc.Ref),
	)
}

func sourceDir(path string, rootdir string, vendordir project.Path) string {
	source := strings.TrimPrefix(path, filepath.Join(rootdir, vendordir.String()))
	return source[1:] // skip leading backslash
}
