// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris || js

package modvendor

import (
	"path"

	"github.com/terramate-io/terramate/os"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
)

func targetPathDir(vendorDir project.Path, modsrc tf.Source) project.Path {
	return project.NewPath(
		path.Join(vendorDir.String(), modsrc.Path, modsrc.Ref),
	)
}

func sourceDir(path os.Path, rootdir os.Path, vendordir project.Path) string {
	return path.TrimPrefix(rootdir.Join(vendordir.String()))
}
