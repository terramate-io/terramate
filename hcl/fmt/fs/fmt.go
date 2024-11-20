// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fs

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/project"
)

const (
	// ErrReadFile is the error kind for any error related to reading the file content.
	ErrReadFile errors.Kind = "failed to read file"
)

// FormatResult represents the result of a formatting operation.
type FormatResult struct {
	path      string
	formatted string
}

// FormatTree will format all Terramate configuration files
// in the given tree starting at the given dir. It will recursively
// navigate on sub directories. Directories starting with "." are ignored.
//
// Only Terramate configuration files will be formatted.
//
// Files that are already formatted are ignored. If all files are formatted
// this function returns an empty result.
//
// All files will be left untouched. To save the formatted result on disk you
// can use FormatResult.Save for each FormatResult.
func FormatTree(root *config.Root, dir project.Path) ([]FormatResult, error) {
	logger := log.With().
		Str("action", "FormatTree").
		Stringer("dir", dir).
		Logger()

	tree, ok := root.Lookup(dir)
	if !ok {
		return nil, errors.E("path %s not found in the loaded configuration", dir)
	}

	for _, fname := range tree.OtherFiles {
		if fname == ".tmskip" {
			logger.Debug().Msg("skip file found: skipping whole subtree")
			return nil, nil
		}
	}

	files := append([]string{}, tree.TerramateFiles...)
	files = append(files, tree.TmGenFiles...)

	sort.Strings(files)

	errs := errors.L()
	results, err := FormatFiles(filepath.Join(root.HostDir(), filepath.FromSlash(dir.String())), files)

	errs.Append(err)

	for _, d := range tree.ChildrenDirs {
		subres, err := FormatTree(root, dir.Join(d))
		if err != nil {
			errs.Append(err)
			continue
		}
		results = append(results, subres...)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})
	return results, nil
}

// FormatFiles will format all the provided Terramate paths.
// Only Terramate configuration files can be reliably formatted with this function.
// If HCL files for a different tool is provided, the result is unpredictable.
//
// Note: The provided file paths can be absolute or relative. If relative, ensure
// working directory is corrected adjusted. The special `-` filename is treated as a
// normal filename, then if it needs to be interpreted as `stdin` this needs to be
// handled separately by the caller.
//
// Files that are already formatted are ignored. If all files are formatted
// this function returns an empty result.
//
// All files will be left untouched. To save the formatted result on disk you
// can use FormatResult.Save for each FormatResult.
func FormatFiles(basedir string, files []string) ([]FormatResult, error) {
	results := []FormatResult{}
	errs := errors.L()

	for _, file := range files {
		fname := file
		if !filepath.IsAbs(file) {
			fname = filepath.Join(basedir, file)
		}
		fileContents, err := os.ReadFile(fname)
		if err != nil {
			errs.Append(errors.E(ErrReadFile, err))
			continue
		}
		currentCode := string(fileContents)
		formatted, err := fmt.Format(currentCode, fname)
		if err != nil {
			errs.Append(err)
			continue
		}
		if currentCode == formatted {
			continue
		}
		results = append(results, FormatResult{
			path:      fname,
			formatted: formatted,
		})
	}
	if err := errs.AsError(); err != nil {
		return nil, err
	}
	return results, nil
}

// Save will save the formatted result on the original file, replacing
// its original contents.
func (f FormatResult) Save() error {
	return os.WriteFile(f.path, []byte(f.formatted), 0644)
}

// Path is the absolute path of the original file.
func (f FormatResult) Path() string {
	return f.path
}

// Formatted is the contents of the original file after formatting.
func (f FormatResult) Formatted() string {
	return f.formatted
}
