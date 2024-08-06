// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package info

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/project"
)

// Pos represents a single position in a source file, by addressing the
// start byte of a unicode character encoded in UTF-8.
//
// Pos is generally used only in the context of a Range, which then defines
// which source file the position is within.
type Pos struct {
	line       int
	column     int
	byteOffset int
}

// Range represents a span of characters between two positions in a source file.
type Range struct {
	hostpath   string
	path       project.Path
	start, end Pos
}

// Ranges is a list of range.
type Ranges []Range

// NewRange creates a new Range from the given [hcl.Range] and the rootdir.
// This function assumes that the filename on the given [hcl.Range] is
// absolute and inside rootdir.
func NewRange(rootdir string, r hcl.Range) Range {
	return Range{
		path:     project.PrjAbsPath(rootdir, r.Filename),
		hostpath: r.Filename,
		start:    NewPos(r.Start),
		end:      NewPos(r.End),
	}
}

// NewPos creates a new Pos from the given [hcl.Pos].
func NewPos(p hcl.Pos) Pos {
	return Pos{
		line:       p.Line,
		column:     p.Column,
		byteOffset: p.Byte,
	}
}

// HostPath is the name of the file into which this range's positions point.
// It is always an absolute path on the host filesystem.
func (r Range) HostPath() string {
	return r.hostpath
}

// Path is the name of the file into which this range's positions point.
// It is always an absolute path relative to the project root.
func (r Range) Path() project.Path {
	return r.path
}

// Start represents the start of the bounds of this range, it is inclusive.
func (r Range) Start() Pos {
	return r.start
}

// End represents the end of the bounds of this range, it is exclusive.
func (r Range) End() Pos {
	return r.end
}

func (r Range) String() string {
	if r.start.line == r.end.line {
		return fmt.Sprintf(
			"%s:%d,%d-%d",
			r.path,
			r.start.line, r.start.column,
			r.end.column,
		)
	}

	return fmt.Sprintf(
		"%s:%d,%d-%d,%d",
		r.path,
		r.start.line, r.start.column,
		r.end.line, r.end.column,
	)
}

// Line is the source code line where this position points. Lines are
// counted starting at 1 and incremented for each newline character
// encountered.
func (p Pos) Line() int {
	return p.line
}

// Column is the source code column where this position points, in
// unicode characters, with counting starting at 1.
//
// Column counts characters as they appear visually, so for example a
// latin letter with a combining diacritic mark counts as one character.
// This is intended for rendering visual markers against source code in
// contexts where these diacritics would be rendered in a single character
// cell. Technically speaking, Column is counting grapheme clusters as
// used in unicode normalization.
func (p Pos) Column() int {
	return p.column
}

// Byte is the byte offset into the file where the indicated character
// begins. This is a zero-based offset to the first byte of the first
// UTF-8 codepoint sequence in the character, and thus gives a position
// that can be resolved _without_ awareness of Unicode characters.
func (p Pos) Byte() int {
	return p.byteOffset
}

// ToHCLRange converts Range to [hcl.Range].
func (r Range) ToHCLRange() hcl.Range {
	return hcl.Range{
		Filename: r.HostPath(),
		Start: hcl.Pos{
			Byte:   r.Start().Byte(),
			Line:   r.Start().Line(),
			Column: r.Start().Column(),
		},
		End: hcl.Pos{
			Byte:   r.End().Byte(),
			Line:   r.End().Line(),
			Column: r.End().Column(),
		},
	}
}

// Paths returns the list of paths of the ranges.
func (rrs Ranges) Paths() project.Paths {
	var paths project.Paths
	for _, r := range rrs {
		paths = append(paths, r.Path())
	}
	return paths
}
