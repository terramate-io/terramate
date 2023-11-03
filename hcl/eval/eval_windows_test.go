// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package eval_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/terramate-io/terramate/test"
	"github.com/zclconf/go-cty/cty"
)

func tmAbspathTestcases(t *testing.T) []testcase {
	tempDir := test.TempDir(t)

	root := root(t)
	return []testcase{
		{
			name: "absolute path is cleaned",
			expr: s(`tm_abspath("%stest\\something")`, root),
			want: want{
				value: cty.StringVal(s(`%stest\something`, root)),
			},
		},
		{
			name: "relative path is appended to basedir",
			expr: `tm_abspath("something")`,
			want: want{
				value: cty.StringVal(s(`%ssomething`, root)),
			},
		},
		{
			name: "relative path is cleaned",
			expr: `tm_abspath("something\\")`,
			want: want{
				value: cty.StringVal(s(`%ssomething`, root)),
			},
		},
		{
			name:    "relative path with multiple levels is appended to basedir",
			expr:    `tm_abspath("a\b\c\d\e")`,
			basedir: tempDir,
			want: want{
				value: cty.StringVal(s(`%s\a\b\c\d\e`, tempDir)),
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
				value: cty.StringVal(s(`%s1`, root)),
			},
		},
	}
}

func root(t *testing.T) string {
	drive := os.Getenv("SYSTEMDRIVE")
	if drive == "" {
		t.Skip("skipping on windows because SYSTEMDRIVE environment is not set")
	}
	return s(`%s\`, drive)
}

func s(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
