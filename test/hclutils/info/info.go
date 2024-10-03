// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package info provides functions useful to create types like [info.Range]
package info

import (
	stdos "os"
	"path/filepath"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/os"
	"github.com/terramate-io/terramate/test/hclutils"
)

// Range builds a [info.Range] for testing purposes.
// On tests we don't know the rootdir at test definition, so
// no rootdir is required. The returned range, if set on an [hcl.Config]
// will have to be fixed with [FixRangesOnConfig] before being
// compared to actual results.
func Range(fname string, start, end hhcl.Pos) info.Range {
	if !filepath.IsAbs(fname) {
		fname = string(stdos.PathSeparator) + fname
	}
	return info.NewRange("/", hhcl.Range{
		Filename: fname,
		Start:    start,
		End:      end,
	})
}

// FixRangesOnConfig fix the ranges on the given HCL config.
// This is necessary since on tests we don't know the sandbox project
// path, so host absolute paths must be updated here.
func FixRangesOnConfig(dir os.Path, cfg hcl.Config) {
	for i := range cfg.Asserts {
		cfg.Asserts[i].Range = FixRange(dir, cfg.Asserts[i].Range)
	}
	for i := range cfg.Generate.Files {
		cfg.Generate.Files[i].Range = FixRange(dir,
			cfg.Generate.Files[i].Range)

		fixRangeOnAsserts(dir, cfg.Generate.Files[i].Asserts)
	}
	for i := range cfg.Generate.HCLs {
		cfg.Generate.HCLs[i].Range = FixRange(dir,
			cfg.Generate.HCLs[i].Range)

		fixRangeOnAsserts(dir, cfg.Generate.HCLs[i].Asserts)
	}
}

// FixRange fix the given range.
// This is necessary since on tests we don't know the sandbox project
// path, so host absolute paths must be updated here.
func FixRange(rootdir os.Path, old info.Range) info.Range {
	// When defining test cases there is no way to know the final
	// absolute paths since sandboxes are dynamic/temporary.
	// So we use relative paths as host paths and make them absolute here.
	var zero info.Range
	if old == zero {
		// ast.Range is a zero value ast.Range, nothing to do
		// avoid panics since the paths are not valid (empty strings).
		return old
	}
	// TODO(i4k): review this!!!!
	filename := rootdir.Join(old.HostPath().String())
	return info.NewRange(rootdir, hclutils.Mkrange(filename,
		hclutils.Start(
			old.Start().Line(),
			old.Start().Column(),
			old.Start().Byte()),
		hclutils.End(
			old.End().Line(),
			old.End().Column(),
			old.End().Byte())))
}

func fixRangeOnAsserts(dir os.Path, asserts []hcl.AssertConfig) {
	for i := range asserts {
		asserts[i].Range = FixRange(dir, asserts[i].Range)
	}
}
