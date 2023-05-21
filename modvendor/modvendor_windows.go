// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package modvendor

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
)

// targetPathDir returns the target path of the module for Windows systems.
// On Windows, the colon (:) is prohibited in path components other than volume,
// then it needs to be replaced by something else when vendoring file:// deps.
func targetPathDir(vendorDir project.Path, modsrc tf.Source) project.Path {
	tpath := modsrc.Path
	if modsrc.PathScheme == "file" {
		// Windows Path in File URI has the form: /<winpath>
		tpath = tpath[1:]
		colonPos := strings.Index(tpath, ":")
		slashPos := strings.Index(tpath, "/")

		// if : is before / (if found)
		// This checks that we replace if:
		//   D:/<etc>
		// But not if:
		//   test/D:/etc
		if colonPos > 0 && (slashPos == -1 || slashPos > colonPos) {
			tpath = tpath[0:colonPos] + "$" + tpath[colonPos+1:]
		}
	}

	return project.NewPath(
		path.Join(vendorDir.String(), tpath, modsrc.Ref),
	)
}

func sourceDir(path string, rootdir string, vendordir project.Path) string {
	source := strings.TrimPrefix(path, filepath.Join(rootdir, vendordir.String()))
	source = source[1:] // skip leading backslash
	colonPos := strings.Index(source, "$")
	slashPos := strings.Index(source, string(os.PathSeparator))
	if colonPos > 0 && (slashPos == -1 || slashPos > colonPos) {
		source = source[0:colonPos] + ":" + source[colonPos+1:]
	}
	return source
}
