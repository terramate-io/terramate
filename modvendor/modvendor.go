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

// Package modvendor provides basic functions and types to support Terraform
// module vendoring.
package modvendor

import (
	"path/filepath"

	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
)

// TargetDir returns the directory for the vendored module source, relative to project
// root.
//
// On Windows, when modsrc.Scheme is "file" it replaces the volume “:“ by `$` because
// `:` is disallowed as path component in such system.
func TargetDir(vendorDir project.Path, modsrc tf.Source) project.Path {
	return targetPathDir(vendorDir, modsrc)
}

// SourceDir returns the source directory from a target directory (installed module).
func SourceDir(path string, rootdir string, vendordir project.Path) string {
	return sourceDir(path, rootdir, vendordir)
}

// AbsVendorDir returns the absolute host path of the vendored module source.
func AbsVendorDir(rootdir string, vendorDir project.Path, modsrc tf.Source) string {
	return filepath.Join(rootdir, filepath.FromSlash(TargetDir(vendorDir, modsrc).String()))
}
