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

func root(t *testing.T) string { return "/"}