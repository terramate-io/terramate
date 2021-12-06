package terrastack

import (
	"io/fs"
	"path/filepath"
)

// Generate will walk all the directories starting from basedir generating
// code for any stack it finds as it goes along
//
// It will return an error if it finds any invalid terrastack configuration files
// of if it can't generate the files properly for some reason.
//
// The provided basedir must be an absolute path to a directory.
func Generate(basedir string) error {
	return filepath.WalkDir(basedir, func(path string, d fs.DirEntry, err error) error {
		return err
	})
}
