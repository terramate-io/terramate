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

// Package hclutils provides test utils related to hcl.
package hclutils

import (
	"path/filepath"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/info"
)

// FixupFiledirOnErrorsFileRanges fix the filename in the ranges of the error list.
func FixupFiledirOnErrorsFileRanges(dir string, errs []error) {
	for _, err := range errs {
		if e, ok := err.(*errors.Error); ok {
			e.FileRange.Filename = filepath.Join(dir, e.FileRange.Filename)
		}
	}
}

// FixRangesOnConfig fix the ranges on the given HCL config.
// This is necessary since on tests we don't know the sandbox project
// path, so host absolute paths must be updated here.
func FixRangesOnConfig(dir string, cfg hcl.Config) {
	for i := range cfg.Asserts {
		cfg.Asserts[i].Range = newRange(dir, cfg.Asserts[i].Range)
	}
	for i := range cfg.Generate.Files {
		cfg.Generate.Files[i].Range = newRange(dir,
			cfg.Generate.Files[i].Range)

		fixRangeOnAsserts(dir, cfg.Generate.Files[i].Asserts)
	}
	for i := range cfg.Generate.HCLs {
		cfg.Generate.HCLs[i].Range = newRange(dir,
			cfg.Generate.HCLs[i].Range)

		fixRangeOnAsserts(dir, cfg.Generate.HCLs[i].Asserts)
	}
}

func fixRangeOnAsserts(dir string, asserts []hcl.AssertConfig) {
	for i := range asserts {
		asserts[i].Range = newRange(dir, asserts[i].Range)
	}
}

// Mkrange builds a file range.
func Mkrange(fname string, start, end hhcl.Pos) hhcl.Range {
	return hhcl.Range{
		Filename: fname,
		Start:    start,
		End:      end,
	}
}

// Start pos of a range.
func Start(line, column, char int) hhcl.Pos {
	return hhcl.Pos{
		Line:   line,
		Column: column,
		Byte:   char,
	}
}

// End pos of a range.
func End(line, column, char int) hhcl.Pos { return Start(line, column, char) }

func newRange(rootdir string, old info.Range) info.Range {
	// When defining test cases there is no way to know the final
	// absolute paths since sandboxes are dynamic/temporary.
	// So we use relative paths as host paths and make them absolute here.
	var zero info.Range
	if old == zero {
		// ast.Range is a zero value ast.Range, nothing to do
		// avoid panics since the paths are not valid (empty strings).
		return old
	}
	filename := filepath.Join(rootdir, old.HostPath())
	return info.NewRange(rootdir, Mkrange(filename,
		Start(old.Start().Line(), old.Start().Column(), old.Start().Byte()),
		End(old.End().Line(), old.End().Column(), old.End().Byte())))
}
