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

package e2etest

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
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
			run   runExpected
			files []file
		}

		testcase struct {
			name   string
			layout []string
			files  []file
			want   want
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
				run: runExpected{
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
				run: runExpected{
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
				run: runExpected{
					Stdout: `Code generation report

Successes:

- /stack
	[+] file.hcl
	[+] file.txt

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.
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
				run: runExpected{
					// TODO(KATCIPIS): add vendor report here.
					Stdout: fmt.Sprintf(`vendor: downloading %s at %s
Code generation report

Successes:

- /stack
	[+] file.hcl
	[+] file.txt

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.
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
	}

	for _, tc := range tcases {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, file := range tcase.files {
				test.WriteFile(t,
					s.RootDir(),
					file.path.String(),
					file.body.String(),
				)
			}

			tmcli := newCLI(t, s.RootDir())
			res := tmcli.run("generate")
			assertRunResult(t, res, tcase.want.run)

			for _, wantFile := range tcase.want.files {
				t.Logf("checking if wanted file %q was created", wantFile.path)

				gotFile := test.ReadFile(t, s.RootDir(), wantFile.path.String())
				test.AssertGenCodeEquals(t, string(gotFile), wantFile.body.String())
			}
		})
	}
}

type str string

func (s str) String() string {
	return string(s)
}
