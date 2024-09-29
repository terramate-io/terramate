// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package modvendor provides basic functions and types to support Terraform
// module vendoring.
package modvendor

import (
	"path/filepath"

	"github.com/terramate-io/terramate/os"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
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
func SourceDir(path os.Path, rootdir os.Path, vendordir project.Path) string {
	return sourceDir(path, rootdir, vendordir)
}

// AbsVendorDir returns the absolute host path of the vendored module source.
func AbsVendorDir(rootdir os.Path, vendorDir project.Path, modsrc tf.Source) os.Path {
	return rootdir.Join(filepath.FromSlash(TargetDir(vendorDir, modsrc).String()))
}
