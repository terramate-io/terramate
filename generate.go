package terrastack

import (
	"fmt"
	"os"
)

// Generate will walk all the directories starting from basedir generating
// code for any stack it finds as it goes along
//
// It will return an error if it finds any invalid terrastack configuration files
// of if it can't generate the files properly for some reason.
//
// The provided basedir must be an absolute path to a directory.
func Generate(basedir string) error {
	// filepath.WalkDir doesn't fail if basedir is a file, so check explicitly
	info, err := os.Lstat(basedir)
	if err != nil {
		return fmt.Errorf("Generate(): invalid base dir %q: %v", basedir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("Generate(): base dir %q is not a directory", basedir)
	}

	return nil
}
