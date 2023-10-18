// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
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

// CopyAll copies all files from dstdir into srcdir.
func CopyAll(dstdir, srcdir string) error {
	return CopyDir(dstdir, srcdir, func(_ string, _ os.DirEntry) bool {
		return true
	})
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
