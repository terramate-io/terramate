//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris || js

package modvendor

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
)

func targetPathDir(vendorDir project.Path, modsrc tf.Source) project.Path {
	return project.NewPath(
		path.Join(vendorDir.String(), modsrc.Path, modsrc.Ref),
	)
}

func sourceDir(path string, rootdir string, vendordir project.Path) string {
	return strings.TrimPrefix(path, filepath.Join(rootdir, vendordir.String()))
}
