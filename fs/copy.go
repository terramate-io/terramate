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

package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

// CopyFilterFunc filters which files/dirs will be copied by CopyDir.
// If the function returns true, the file/dir is copied.
// If it returns false, the file/dir is ignored.
type CopyFilterFunc func(path string, entry os.DirEntry) bool

// CopyDir will copy srcdir to destdir.
// It will copy all dirs and files recursively.
// The destdir provided does not need to exist, it will be created.
// The provided filter function allows to filter which files/directories get copied.
func CopyDir(destdir, srcdir string, filter CopyFilterFunc) error {
	const (
		createDirMode = 0755
	)

	entries, err := os.ReadDir(srcdir)
	if err != nil {
		return errors.E(err, "reading src dir")
	}

	createdDir := false
	createDir := func() error {
		if createdDir {
			return nil
		}
		if err := os.MkdirAll(destdir, createDirMode); err != nil {
			return errors.E(err, "creating dest dir")
		}
		createdDir = true
		return nil
	}

	for _, entry := range entries {
		if !filter(srcdir, entry) {
			continue
		}

		srcpath := filepath.Join(srcdir, entry.Name())
		destpath := filepath.Join(destdir, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(destpath, srcpath, filter); err != nil {
				return errors.E(err, "copying src to dest dir")
			}
			continue
		}

		// Only create dir if there is a file to copy to it or if some of
		// its subdirs have a file to copy on it.
		if err := createDir(); err != nil {
			return err
		}

		if err := copyFile(destpath, srcpath); err != nil {
			return errors.E(err, "copying src to dest file")
		}
	}

	return nil
}

func copyFile(destfile, srcfile string) error {
	src, err := os.Open(srcfile)
	if err != nil {
		return errors.E(err, "opening source file")
	}
	defer closeFile(src)
	dest, err := os.Create(destfile)
	if err != nil {
		return errors.E(err, "creating dest file")
	}
	defer closeFile(dest)
	_, err = io.Copy(dest, src)
	return err
}

func closeFile(file *os.File) {
	err := file.Close()
	if err != nil {
		log.Warn().
			Str("file", file.Name()).
			Err(err).
			Msg("closing file ")
	}
}
