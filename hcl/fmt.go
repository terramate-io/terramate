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

package hcl

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
)

// FormatResult represents the result of a formatting operation.
type FormatResult struct {
	path      string
	formatted string
}

// Format will format the given source code. It returns an error if the given
// source is invalid HCL.
func Format(src, filename string) (string, error) {
	p := hclparse.NewParser()
	_, diags := p.ParseHCL([]byte(src), filename)
	if err := errors.L(diags).AsError(); err != nil {
		return "", errors.E(ErrHCLSyntax, err)
	}
	// For now we just use plain hclwrite.Format
	// but we plan on customizing formatting in the near future.
	return string(hclwrite.Format([]byte(src))), nil
}

// FormatFile will format the given HCL file.
func FormatFile(filepath string) (string, error) {
	body, err := os.ReadFile(filepath)
	if err != nil {
		return "", errors.E(errFormatFile, err)
	}
	return Format(string(body), filepath)
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
// All files will be left untouched.
func FormatTree(dir string) ([]FormatResult, error) {
	logger := log.With().
		Str("action", "hcl.FormatTree()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing terramate files")

	files, err := listTerramateFiles(dir)
	if err != nil {
		return nil, errors.E(errFormatTree, err)
	}

	results := []FormatResult{}
	errs := errors.L()

	for _, f := range files {
		logger := log.With().
			Str("file", f).
			Logger()

		logger.Trace().Msg("formatting file")
		file := filepath.Join(dir, f)
		formatted, err := FormatFile(file)
		if err != nil {
			errs.Append(err)
			continue
		}

		results = append(results, FormatResult{
			path:      file,
			formatted: formatted,
		})
	}

	dirs, err := listTerramateDirs(dir)
	if err != nil {
		errs.Append(err)
		return nil, errors.E(errFormatTree, errs)
	}

	for _, d := range dirs {
		logger := log.With().
			Str("subdir", d).
			Logger()

		logger.Trace().Msg("recursively formatting")
		subres, err := FormatTree(filepath.Join(dir, d))
		if err != nil {
			errs.Append(err)
			continue
		}
		results = append(results, subres...)
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

const (
	errFormatTree errors.Kind = "formatting tree"
	errFormatFile errors.Kind = "formatting file"
)
