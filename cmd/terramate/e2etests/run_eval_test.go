// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import (
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunEval(t *testing.T) {
	// tests the `terramate run --eval -- cmd containing '${tm_upper("hcl")}'`

	type testcase struct {
		name   string
		layout []string
		eval   bool
		args   []string
		want   runExpected
	}

	for _, tc := range []testcase{
		{
			name:   "no eval -- ignores $ and other HCL templating symbols",
			layout: []string{`s:stack`},
			eval:   false,
			args: []string{
				`test ${tm_upper("hcl")}`,
				`%{ for i in tm_range(5) ~} some ${i} %{endfor}`,
			},
			want: runExpected{
				Stdout: "test ${tm_upper(\"hcl\")} %{ for i in tm_range(5) ~} some ${i} %{endfor}\n",
			},
		},
		{
			name:   "with no interpolation, return as is",
			layout: []string{`s:stack:id=stackid`},
			eval:   true,
			args: []string{
				`terramate.stack.id`,
			},
			want: runExpected{
				Stdout: "terramate.stack.id\n",
			},
		},
		{
			name:   "eval of interpolation supports terramate metadata",
			layout: []string{`s:stack:id=stackid`},
			eval:   true,
			args: []string{
				`${terramate.stack.id}`,
			},
			want: runExpected{
				Stdout: "stackid\n",
			},
		},
		{
			name:   "function interpolation and other HCL templating symbols",
			layout: []string{`s:stack`},
			eval:   true,
			args: []string{
				`test ${tm_upper("hcl")}`,
				`%{ for i in tm_range(5) ~} some ${i} %{endfor}`,
			},
			want: runExpected{
				Stdout: "test HCL some 0 some 1 some 2 some 3 some 4 \n",
			},
		},
		{
			name:   "no eval -- ignores escaped $ and other escaped HCL templating symbols",
			layout: []string{`s:stack`},
			eval:   true,
			args: []string{
				`test $${tm_upper(\"hcl\")}`,
				`%%{ for i in tm_range(5) ~} some $${i} %%{endfor}`,
			},
			want: runExpected{
				Stdout: "test ${tm_upper(\"hcl\")} %{ for i in tm_range(5) ~} some ${i} %{endfor}\n",
			},
		},
		{
			// WHY? When using --eval, each argument is interpreted as
			// a raw HCL string content, which means it's the same as:
			//   OQUOTE ARG CQUOTE
			// Then if the user wants to include a literal quote, it must be
			// escaped:
			//   OQUOTE \" CQUOTE
			name:   "malformed hcl string",
			layout: []string{`s:stack`},
			eval:   true,
			args: []string{
				`"`,
			},
			want: runExpected{
				Status:      1,
				StderrRegex: `parsing expression`,
			},
		},
		{
			name:   "escaped quote return as is",
			layout: []string{`s:stack`},
			eval:   true,
			args: []string{
				`\"`,
			},
			want: runExpected{
				Stdout: "\"\n",
			},
		},
		{
			name:   "fs functions are not exposed",
			layout: []string{`s:stack`},
			eval:   true,
			args: []string{
				`${tm_abspath(".")}`,
			},
			want: runExpected{
				Status:      1,
				StderrRegex: `There is no function named`,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := sandbox.New(t)
			if len(tc.layout) == 0 {
				t.Fatal("please set tc.layout, so it run at least 1 stack")
			}
			s.BuildTree(tc.layout)
			git := s.Git()
			git.CommitAll("everything")
			tmCli := newCLI(t, s.RootDir())
			cmd := []string{`run`}
			if tc.eval {
				cmd = append(cmd, `--eval`)
			}

			// WHY?
			// we are executing: terramate run --eval -- <helper test binary> echo <arg1, ..., argN>
			// the problem is that --eval requires each argument to be a valid
			// HCL string but then on Windows we cannot just paste the test binary
			// path here because it's not a valid HCL string. Example:
			//   terramate run --eval -- C:\Users\i4k\test.exe arg1 arg2
			// This will construct the HCL expression below:
			//   "C:\Users\i4k\test.exe"
			// and then the HCL parser fails because it's gonna interpret \U as an
			// invalid unicode sequence.
			//
			// The user will have to properly escape it like:
			//   terramate run --eval -- C:\\Users\\i4k\\test.exe arg1 arg2
			//
			// To avoid escaping everything, we are prepending the helper
			// binary directory to the PATH environment and then invoking
			// just the basename.
			//   PATH=$HELPER_DIR:$PATH terramate run --eval -- test.exe echo arg1 arg2
			testHelperDir := filepath.Dir(testHelperBin)
			testHelperName := filepath.Base(testHelperBin)
			tmCli.prependToPath(testHelperDir)
			cmd = append(cmd, "--", testHelperName, "echo")
			cmd = append(cmd, tc.args...)
			assertRunResult(t, tmCli.run(cmd...), tc.want)
		})
	}
}
