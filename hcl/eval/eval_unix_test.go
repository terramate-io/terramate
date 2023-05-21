// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package eval_test

import (
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func tmAbspathTestcases(t *testing.T) []testcase {
	tempDir := t.TempDir()

	return []testcase{
		{
			name: "absolute path is cleaned",
			expr: `tm_abspath("/test//something")`,
			want: want{
				value: cty.StringVal("/test/something"),
			},
		},
		{
			name: "relative path is appended to basedir",
			expr: `tm_abspath("something")`,
			want: want{
				value: cty.StringVal("/something"),
			},
		},
		{
			name: "relative path is cleaned",
			expr: `tm_abspath("something//")`,
			want: want{
				value: cty.StringVal("/something"),
			},
		},
		{
			name: "relative path with multiple levels is appended to basedir",
			expr: `tm_abspath("a/b/c/d/e")`,
			want: want{
				value: cty.StringVal("/a/b/c/d/e"),
			},
		},
		{
			name:    "empty path returns the basedir",
			expr:    `tm_abspath("")`,
			basedir: tempDir,
			want: want{
				value: cty.StringVal(tempDir),
			},
		},
		{
			name: "argument is a number - works ... mimicking terraform abspath()",
			expr: `tm_abspath(1)`,
			want: want{
				value: cty.StringVal("/1"),
			},
		},
	}
}

func root(_ *testing.T) string { return "/" }
