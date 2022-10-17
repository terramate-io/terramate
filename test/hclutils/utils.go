// Package hclutils provides test utils related to hcl.
package hclutils

import (
	"path/filepath"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
)

// FixupFiledirOnErrorsFileRanges fix the filename in the ranges of the error list.
func FixupFiledirOnErrorsFileRanges(dir string, errs []error) {
	for _, err := range errs {
		if e, ok := err.(*errors.Error); ok {
			e.FileRange.Filename = filepath.Join(dir, e.FileRange.Filename)
		}
	}
}

// FixupFiledirOnConfig fix up the filename on the config origins.
func FixupFiledirOnConfig(dir string, cfg hcl.Config) {
	for i := range cfg.Asserts {
		cfg.Asserts[i].Origin = filepath.Join(dir, cfg.Asserts[i].Origin)
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
