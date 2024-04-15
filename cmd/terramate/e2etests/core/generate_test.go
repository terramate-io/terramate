// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/modvendor"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

// Most of the code generation behavior is tested
// through the generate pkg. Here we test the integration of code generation
// and vendoring. The way the integration is done has a good chance of
// changing so testing this in an e2e manner makes it less liable to
// break because of structural changes.

func TestGenerate(t *testing.T) {
	t.Parallel()

	type (
		file struct {
			path project.Path
			body fmt.Stringer
		}

		want struct {
			run   RunExpected
			files []file
		}

		testcase struct {
			name             string
			layout           []string
			files            []file
			detailedExitCode bool
			want             want
		}
	)

	const (
		noCodegenMsg = "Nothing to do, generated code is up to date\n"
		filename     = "test.txt"
		content      = "generate-tests"
	)

	p := project.NewPath
	gitSource := newGitSource(t, filename, content)
	gitSource += "?ref=main"
	modsrc := test.ParseSource(t, gitSource)
	defaultVendor := project.NewPath("/modules")
	vendorTargetDir := modvendor.TargetDir(defaultVendor, modsrc)

	tmVendorCallExpr := func() string {
		return fmt.Sprintf(`tm_vendor("%s")`, gitSource)
	}

	tcases := []testcase{
		{
			name: "no stacks",
			want: want{
				run: RunExpected{
					Stdout: noCodegenMsg,
				},
			},
		},
		{
			name: "stacks with no codegen",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			want: want{
				run: RunExpected{
					Stdout: noCodegenMsg,
				},
			},
		},
		{
			name: "stacks with no codegen and --detailed-exit-code",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			detailedExitCode: true,
			want: want{
				run: RunExpected{
					Stdout: noCodegenMsg,
				},
			},
		},
		{
			name: "generate file and hcl",
			layout: []string{
				"s:stack",
			},
			files: []file{
				{
					path: p("/config.tm"),
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Str("a", "hi"),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Str("content", "hi"),
						),
					),
				},
			},
			want: want{
				run: RunExpected{
					Stdout: `Code generation report

Successes:

- /stack
	[+] file.hcl
	[+] file.txt

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.
`,
				},
				files: []file{
					{
						path: p("/stack/file.hcl"),
						body: Doc(
							Str("a", "hi"),
						),
					},
					{
						path: p("/stack/file.txt"),
						body: str("hi"),
					},
				},
			},
		},
		{
			name: "generate file and hcl with --detailed-exit-code",
			layout: []string{
				"s:stack",
			},
			detailedExitCode: true,
			files: []file{
				{
					path: p("/config.tm"),
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Str("a", "hi"),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Str("content", "hi"),
						),
					),
				},
			},
			want: want{
				run: RunExpected{
					IgnoreStdout: true,
					Status:       2,
				},
				files: []file{
					{
						path: p("/stack/file.hcl"),
						body: Doc(
							Str("a", "hi"),
						),
					},
					{
						path: p("/stack/file.txt"),
						body: str("hi"),
					},
				},
			},
		},
		{
			name: "generate file and hcl with tm_vendor",
			layout: []string{
				"s:stack",
			},
			files: []file{
				{
					path: p("/config.tm"),
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("vendor", tmVendorCallExpr()),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Expr("content", tmVendorCallExpr()),
						),
					),
				},
			},
			want: want{
				run: RunExpected{
					Stdout: fmt.Sprintf(`vendor: downloading %s at %s
Code generation report

Successes:

- /stack
	[+] file.hcl
	[+] file.txt

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.
Vendor report:

[+] %s
    ref: %s
    dir: %s
`, gitSource, vendorTargetDir, modsrc.URL, modsrc.Ref, vendorTargetDir),
				},
				files: []file{
					{
						path: p("/stack/file.hcl"),
						body: Doc(
							Str("vendor", ".."+vendorTargetDir.String()),
						),
					},
					{
						path: p("/stack/file.txt"),
						body: str(".." + vendorTargetDir.String()),
					},
				},
			},
		},
		{
			name: "generate works but vendoring fails will exit with 1",
			layout: []string{
				"s:stack",
			},
			files: []file{
				{
					path: p("/config.tm"),
					body: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/terramate-io/unknown/will/fail?ref=fail")`),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Expr("content", `tm_vendor("github.com/terramate-io/unknown/will/fail?ref=fail")`),
						),
					),
				},
			},
			want: want{
				// We are not interested on checking the specific output
				// or the generated files in this case, just ensuring
				// the correct status code.
				run: RunExpected{
					Status:       1,
					IgnoreStdout: true,
				},
			},
		},
		{
			name: "generate_hcl of different stacks dont persist",
			layout: []string{
				"s:stack1",
				"s:stack2",
			},
			files: []file{
				{
					path: p("/config.tm"),
					body: Doc(
						Globals(
							Expr("name", "terramate.stack.name"),
						),
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("a", "global.name"),
							),
						),
					),
				},
			},
			want: want{
				run: RunExpected{
					Stdout: `Code generation report

Successes:

- /stack1
	[+] file.hcl

- /stack2
	[+] file.hcl

Hint: '+', '~' and '-' mean the file was created, changed and deleted, respectively.
`,
				},
				files: []file{
					{
						path: p("/stack1/file.hcl"),
						body: Doc(
							Str("a", "stack1"),
						),
					},
					{
						path: p("/stack2/file.hcl"),
						body: Doc(
							Str("a", "stack2"),
						),
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.NoGit(t, true)
			s.BuildTree(tcase.layout)

			for _, file := range tcase.files {
				test.WriteFile(t,
					s.RootDir(),
					file.path.String(),
					file.body.String(),
				)
			}

			tmcli := NewCLI(t, s.RootDir())
			args := []string{"generate"}
			if tcase.detailedExitCode {
				args = append(args, "--detailed-exit-code")
			}
			res := tmcli.Run(args...)
			AssertRunResult(t, res, tcase.want.run)

			for _, wantFile := range tcase.want.files {
				t.Logf("checking if wanted file %q was created", wantFile.path)

				gotFile := test.ReadFile(t, s.RootDir(), wantFile.path.String())
				test.AssertGenCodeEquals(t, string(gotFile), wantFile.body.String())
			}

			if tcase.want.run.Status == 0 {
				// On success if we re-generate it should also work and always
				// give a "nothing to do" message
				t.Run("regenerate", func(t *testing.T) {
					res := tmcli.Run("generate")
					AssertRunResult(t, res, RunExpected{
						Stdout: noCodegenMsg,
					})
				})
			}
		})
	}
}

func TestGenerateIgnoresWorkingDirectory(t *testing.T) {
	t.Parallel()
	wantStdout := generate.Report{
		Successes: []generate.Result{
			{
				Dir: project.NewPath("/"),
				Created: []string{
					"root.stacks.txt",
				},
			},
			{
				Dir: project.NewPath("/stacks/stack-1"),
				Created: []string{
					"stack.hcl", "stack.name.txt",
				},
			},
			{
				Dir: project.NewPath("/stacks/stack-2"),
				Created: []string{
					"stack.hcl", "stack.name.txt",
				},
			},
		},
	}.Full() + "\n"

	configStr := Doc(
		GenerateFile(
			Labels("stack.name.txt"),
			Expr("content", "terramate.stack.name"),
		),
		GenerateHCL(
			Labels("stack.hcl"),
			Content(
				Expr("name", "terramate.stack.name"),
				Expr("path", "terramate.stack.path"),
			),
		),
		GenerateFile(
			Labels("/root.stacks.txt"),
			Expr("context", "root"),
			Str("content", "stack terramate.stacks.list[0]"),
		),
	).String()

	runFromDir := func(t *testing.T, wd string) {
		t.Run(fmt.Sprintf("terramate -C %s generate", wd), func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
			s.BuildTree([]string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			})

			s.RootEntry().CreateFile(
				config.DefaultFilename,
				configStr,
			)

			tmcli := NewCLI(t, filepath.Join(s.RootDir(), wd))
			res := tmcli.Run("generate")
			expected := RunExpected{
				Stdout: wantStdout,
			}
			AssertRunResult(t, res, expected)
		})
	}

	runFromDir(t, "/")
	runFromDir(t, "/stacks")
	runFromDir(t, "/stacks/stack-1")
}

type str string

func (s str) String() string {
	return string(s)
}
