// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	stdfmt "fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/hcl/fmt"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestFormatRecursively(t *testing.T) {
	t.Parallel()

	const unformattedTmFile = `
globals {
name = "name"
		description = "desc"
	test = true
	}
	`
	const unformattedTmGenFile = `
block {
a = 1
  name = "test"
}
`
	formattedTmContent, err := fmt.Format(unformattedTmFile, "")
	assert.NoError(t, err)
	formattedTmGenContent := `
block {
  a    = 1
  name = "test"
}
`
	s := sandbox.New(t)
	s.BuildTree([]string{
		"f:enable.tmgen.tm:" + Terramate(
			Config(
				Expr("experiments", `["tmgen"]`),
			),
		).String(),
	})
	cli := NewCLI(t, s.RootDir())

	t.Run("checking succeeds when there is no Terramate files", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt", "--check"), RunExpected{})
	})

	t.Run("formatting succeeds when there is no Terramate files", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt"), RunExpected{})
	})

	sprintf := stdfmt.Sprintf
	writeUnformattedFiles := func() {
		s.BuildTree([]string{
			sprintf("f:globals.tm:%s", unformattedTmFile),
			sprintf("f:test.hcl.tmgen:%s", unformattedTmGenFile),
			sprintf("f:another-stacks/globals.tm.hcl:%s", unformattedTmFile),
			sprintf("f:another-stacks/stack-1/globals.tm.hcl:%s", unformattedTmFile),
			sprintf("f:another-stacks/stack-2/globals.tm.hcl:%s", unformattedTmFile),
			sprintf("f:stacks/globals.tm:%s", unformattedTmFile),
			sprintf("f:stacks/test.hcl.tmgen:%s", unformattedTmGenFile),
			sprintf("f:stacks/stack-1/globals.tm:%s", unformattedTmFile),
			sprintf("f:stacks/stack-2/globals.tm:%s", unformattedTmFile),

			// dir below must always be ignored
			`f:skipped-dir/.tmskip:`,
			sprintf("f:skipped-dir/globals.tm:%s", unformattedTmFile),
		})
	}

	writeUnformattedFiles()

	wantedTmFiles := []string{
		"globals.tm",
		"another-stacks/globals.tm.hcl",
		"another-stacks/stack-1/globals.tm.hcl",
		"another-stacks/stack-2/globals.tm.hcl",
		"stacks/globals.tm",
		"stacks/stack-1/globals.tm",
		"stacks/stack-2/globals.tm",
	}
	wantedTmGenFiles := []string{
		"test.hcl.tmgen",
		"stacks/test.hcl.tmgen",
	}
	filesListOutput := func(tmFiles []string, tmGenFiles []string) string {
		portableFiles := []string{}
		files := append(append([]string{}, tmFiles...), tmGenFiles...)
		sort.Strings(files)
		for _, f := range files {
			portableFiles = append(portableFiles, filepath.FromSlash(f))
		}
		return strings.Join(portableFiles, "\n") + "\n"
	}
	wantedFilesStr := filesListOutput(wantedTmFiles, wantedTmGenFiles)

	assertFileContents := func(t *testing.T, path string, want string) {
		t.Helper()
		got := s.RootEntry().ReadFile(path)
		assert.EqualStrings(t, want, string(got))
	}

	assertWantedFilesContents := func(t *testing.T, wantTm, wantTmGen string) {
		t.Helper()

		for _, file := range wantedTmFiles {
			assertFileContents(t, file, wantTm)
		}
		for _, file := range wantedTmGenFiles {
			assertFileContents(t, file, wantTmGen)
		}
	}

	t.Run("checking fails with unformatted files", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt", "--check"), RunExpected{
			Status: 1,
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, unformattedTmFile, unformattedTmGenFile)
	})

	t.Run("--detailed-exit-code returns status=2 when used on unformatted files - do modify the files", func(t *testing.T) {
		// see: https://github.com/terramate-io/terramate/issues/1649
		writeUnformattedFiles()
		AssertRunResult(t, cli.Run("fmt", "--detailed-exit-code"), RunExpected{
			Status: 2,
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})

	t.Run("checking fails with unformatted files on subdirs", func(t *testing.T) {
		writeUnformattedFiles()
		subdir := filepath.Join(s.RootDir(), "another-stacks")
		cli := NewCLI(t, subdir)
		AssertRunResult(t, cli.Run("fmt", "--check"), RunExpected{
			Status: 1,
			Stdout: filesListOutput([]string{
				"globals.tm.hcl",
				"stack-1/globals.tm.hcl",
				"stack-2/globals.tm.hcl",
			}, nil),
		})
		assertWantedFilesContents(t, unformattedTmFile, unformattedTmGenFile)
	})

	t.Run("update unformatted files in place", func(t *testing.T) {
		writeUnformattedFiles()
		AssertRunResult(t, cli.Run("fmt"), RunExpected{
			Stdout: wantedFilesStr,
		})
		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})

	t.Run("checking succeeds when all files are formatted", func(t *testing.T) {
		writeUnformattedFiles()
		AssertRunResult(t, cli.Run("fmt"), RunExpected{IgnoreStdout: true})
		AssertRunResult(t, cli.Run("fmt", "--check"), RunExpected{})
		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})

	t.Run("--detailed-exit-code returns status=0 when all files are formatted", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt", "--detailed-exit-code"), RunExpected{})
		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})

	t.Run("--check and --detailed-exit-code conflict", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt", "--detailed-exit-code", "--check"), RunExpected{
			Status:      1,
			StderrRegex: "--check conflicts with --detailed-exit-code",
		})
	})

	t.Run("formatting succeeds when all files are formatted", func(t *testing.T) {
		AssertRunResult(t, cli.Run("fmt"), RunExpected{})
		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})

	t.Run("update unformatted files in subdirs", func(t *testing.T) {
		writeUnformattedFiles()

		anotherStacks := filepath.Join(s.RootDir(), "another-stacks")
		cli := NewCLI(t, anotherStacks)
		AssertRunResult(t, cli.Run("fmt"), RunExpected{
			Stdout: filesListOutput([]string{
				"globals.tm.hcl",
				"stack-1/globals.tm.hcl",
				"stack-2/globals.tm.hcl",
			}, nil),
		})

		assertFileContents(t, "another-stacks/globals.tm.hcl", formattedTmContent)
		assertFileContents(t, "another-stacks/stack-1/globals.tm.hcl", formattedTmContent)
		assertFileContents(t, "another-stacks/stack-2/globals.tm.hcl", formattedTmContent)

		assertFileContents(t, "globals.tm", unformattedTmFile)
		assertFileContents(t, "stacks/globals.tm", unformattedTmFile)
		assertFileContents(t, "stacks/stack-1/globals.tm", unformattedTmFile)
		assertFileContents(t, "stacks/stack-2/globals.tm", unformattedTmFile)

		stacks := filepath.Join(s.RootDir(), "stacks")
		cli = NewCLI(t, stacks)
		AssertRunResult(t, cli.Run("fmt"), RunExpected{
			Stdout: filesListOutput([]string{
				"globals.tm",
				"stack-1/globals.tm",
				"stack-2/globals.tm",
				"test.hcl.tmgen",
			}, nil),
		})

		assertFileContents(t, "another-stacks/globals.tm.hcl", formattedTmContent)
		assertFileContents(t, "another-stacks/stack-1/globals.tm.hcl", formattedTmContent)
		assertFileContents(t, "another-stacks/stack-2/globals.tm.hcl", formattedTmContent)
		assertFileContents(t, "stacks/globals.tm", formattedTmContent)
		assertFileContents(t, "stacks/stack-1/globals.tm", formattedTmContent)
		assertFileContents(t, "stacks/stack-2/globals.tm", formattedTmContent)

		assertFileContents(t, "globals.tm", unformattedTmFile)

		cli = NewCLI(t, s.RootDir())
		AssertRunResult(t, cli.Run("fmt"), RunExpected{
			Stdout: filesListOutput([]string{"globals.tm"}, []string{"test.hcl.tmgen"}),
		})

		assertWantedFilesContents(t, formattedTmContent, formattedTmGenContent)
	})
}

func TestFmtFiles(t *testing.T) {
	type want struct {
		layout []string
		res    RunExpected
	}
	type testcase struct {
		name     string
		layout   []string
		files    []string
		check    bool
		stdin    string
		absPaths bool
		want     want
	}

	for _, tc := range []testcase{
		{
			name:  "non-existent file",
			files: []string{"non-existent.tm"},
			want: want{
				res: RunExpected{
					StderrRegex: string(fmt.ErrReadFile),
					Status:      1,
				},
			},
		},
		{
			name: "single file",
			layout: []string{
				`f:example.tm:terramate {
					    config{}
						}`,
			},
			files: []string{"example.tm"},
			want: want{
				res: RunExpected{
					Stdout: nljoin("example.tm"),
				},
				layout: []string{
					`f:example.tm:terramate {
  config {}
}`,
				},
			},
		},
		{
			name: "multiple files",
			layout: []string{
				`f:example1.tm:terramate {
					    config{}
						}`,
				`f:example2.tm:terramate {
							config{}
							}`,
			},
			files: []string{"example1.tm", "example2.tm"},
			want: want{
				res: RunExpected{
					Stdout: nljoin("example1.tm", "example2.tm"),
				},
				layout: []string{
					`f:example1.tm:terramate {
  config {}
}`,
					`f:example2.tm:terramate {
  config {}
}`,
				},
			},
		},
		{
			name: "multiple files with --check",
			layout: []string{
				`f:example1.tm:terramate {
					    config{}
						}`,
				`f:example2.tm:terramate {
							config{}
							}`,
			},
			files: []string{"example1.tm", "example2.tm"},
			check: true,
			want: want{
				res: RunExpected{
					Stdout: nljoin("example1.tm", "example2.tm"),
					Status: 1,
				},
			},
		},
		{
			name: "multiple files with absolute path",
			layout: []string{
				`f:example1.tm:terramate {
					    config{}
						}`,
				`f:example2.tm:terramate {
							config{}
							}`,
			},
			absPaths: true,
			files:    []string{"example1.tm", "example2.tm"},
			want: want{
				res: RunExpected{
					Stdout: nljoin("example1.tm", "example2.tm"),
				},
				layout: []string{
					`f:example1.tm:terramate {
  config {}
}`,
					`f:example2.tm:terramate {
  config {}
}`,
				},
			},
		},
		{
			name:  "format stdin",
			files: []string{"-"},
			stdin: `stack {
name="name"
  description = "desc"
			}`,
			want: want{
				res: RunExpected{
					Stdout: `stack {
  name        = "name"
  description = "desc"
}`,
				},
			},
		},
		{
			name:  "format stdin with multiple blocks",
			files: []string{"-"},
			stdin: `stack {
name="name"
  description = "desc"
			}
			
			
			
			generate_file    "a.txt" {
				content = "a"
			}
			
			
`,
			want: want{
				res: RunExpected{
					Stdout: `stack {
  name        = "name"
  description = "desc"
}



generate_file "a.txt" {
  content = "a"
}


`,
				},
			},
		},
		{
			name:  "format stdin without content",
			files: []string{"-"},
		},
		{
			name:  "format stdin with --check",
			files: []string{"-"},
			stdin: `stack {
name="name"
  description = "desc"
			}`,
			check: true,
			want: want{
				res: RunExpected{
					Status: 1,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := sandbox.NoGit(t, true)
			s.BuildTree(tc.layout)
			cli := NewCLI(t, s.RootDir())
			files := tc.files
			if tc.absPaths {
				for i, f := range files {
					files[i] = filepath.Join(s.RootDir(), f)
				}
			}
			args := []string{"fmt"}
			if tc.check {
				args = append(args, "--check")
			}
			args = append(args, files...)
			var result RunResult
			if len(files) == 1 && files[0] == "-" {
				result = cli.RunWithStdin(tc.stdin, args...)
			} else {
				result = cli.Run(args...)
			}
			AssertRunResult(t, result, tc.want.res)
			s.AssertTree(tc.want.layout)
		})
	}
}
