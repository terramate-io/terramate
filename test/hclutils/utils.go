// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package hclutils provides test utils related to hcl.
package hclutils

import (
	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/os"
)

// FixupFiledirOnErrorsFileRanges fix the filename in the ranges of the error list.
func FixupFiledirOnErrorsFileRanges(dir os.Path, errs []error) {
	for _, err := range errs {
		if e, ok := err.(*errors.Error); ok {
			e.FileRange.Filename = dir.Join(e.FileRange.Filename).String()
		}
	}
}

// Mkrange builds a file range.
func Mkrange(fname os.Path, start, end hhcl.Pos) hhcl.Range {
	return hhcl.Range{
		Filename: fname.String(),
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
