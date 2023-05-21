// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package hclutils provides test utils related to hcl.
package hclutils

import (
	"path/filepath"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
)

// FixupFiledirOnErrorsFileRanges fix the filename in the ranges of the error list.
func FixupFiledirOnErrorsFileRanges(dir string, errs []error) {
	for _, err := range errs {
		if e, ok := err.(*errors.Error); ok {
			e.FileRange.Filename = filepath.Join(dir, e.FileRange.Filename)
		}
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
