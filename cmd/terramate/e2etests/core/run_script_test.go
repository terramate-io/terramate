// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunScript(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name       string
		layout     []string
		runScript  []string
		workingDir string
		want       RunExpected
	}

	terramateConfig :=
		`f:terramate.tm:
	  terramate {
		config {
		  experiments = ["scripts"]
		}
	  }`

	for _, tc := range []testcase{
		{
			name: "aborts if scripts are not enabled",
			layout: []string{
				"s:stack-a",
			},
			runScript: []string{"somescript"},
			want: RunExpected{
				StderrRegex: " feature is not enabled",
				Status:      1,
			},
		},
		{
			name: "script defined in stack should run successfully",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello"]
				  }
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"somescript"},
			want: RunExpected{
				StderrRegexes: []string{
					"Script 0 at /stack-a/script.tm:.* having 2 job\\(s\\)",
					"/stack-a \\(script:0 job:0.0\\)> echo hello",
					"/stack-a \\(script:0 job:1.0\\)> echo some message",
				},
				StdoutRegexes: []string{
					"hello",
					"some message",
				},
			},
		},
		{
			name: "unknown script should return exit code",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello"]
				  }
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"someunknownscript"},
			want: RunExpected{
				Stderr: "script not found: someunknownscript\n",
				Status: 1,
			},
		},
		{
			name: "script defined in stack should run also in child stacks",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				"s:stack-a/child",
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"somescript"},
			want: RunExpected{
				StderrRegexes: []string{
					"Script 0 at /stack-a/script.tm:.* having 1 job\\(s\\)",
					"/stack-a \\(script:0 job:0.0\\)> echo some message",
					"/stack-a/child \\(script:0 job:0.0\\)> echo some message",
				},
				StdoutRegexes: []string{
					"some message",
				},
			},
		},
		{
			name: "run-script --tags should only run on the relevant stacks",
			layout: []string{
				terramateConfig,
				`s:stack-a:tags=["terraform"]`,
				`s:stack-a/child:tags=["kubernetes"]`,
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"--tags=kubernetes", "somescript"},
			want: RunExpected{
				Stdout: `some message`,
				Stderr: "Script 0 at /stack-a/script.tm:2,5-7,6 having 1 job(s)\n" +
					"/stack-a/child (script:0 job:0.0)> echo some message\n",
				FlattenStdout: true,
			},
		},
		{
			name: "run-script --no-tags should only run on the relevant stacks",
			layout: []string{
				terramateConfig,
				`s:stack-a:tags=["terraform"]`,
				`s:stack-a/child:tags=["kubernetes"]`,
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"--no-tags=terraform", "somescript"},
			want: RunExpected{
				Stdout: `some message`,
				Stderr: "Script 0 at /stack-a/script.tm:2,5-7,6 having 1 job(s)\n" +
					"/stack-a/child (script:0 job:0.0)> echo some message\n",
				FlattenStdout: true,
			},
		},
		{
			name:       "run-script --no-recursive should only run on the relevant stacks",
			workingDir: "stack-a",
			layout: []string{
				terramateConfig,
				`s:stack-a:tags=["terraform"]`,
				`s:stack-a/child:tags=["kubernetes"]`,
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"--no-recursive", "somescript"},
			want: RunExpected{
				Stdout: `some message`,
				Stderr: "Script 0 at /stack-a/script.tm:2,5-7,6 having 1 job(s)\n" +
					"/stack-a (script:0 job:0.0)> echo some message\n",
				FlattenStdout: true,
			},
		},
		{
			name:       "run-script --dry-run should not execute commands",
			workingDir: "stack-a",
			layout: []string{
				terramateConfig,
				`s:stack-a:tags=["terraform"]`,
				`f:stack-a/globals.tm:
				globals {
				  message = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "${global.message}"]
				  }
				}`,
				"s:stack-b",
			},
			runScript: []string{"--dry-run", "somescript"},
			want: RunExpected{
				Stdout: "",
				Stderr: "This is a dry run, commands will not be executed.\n" +
					"Script 0 at /stack-a/script.tm:2,5-7,6 having 1 job(s)\n" +
					"/stack-a (script:0 job:0.0)> echo some message\n",
			},
		},
		{
			name: "complex before/after keeps script commands in order",
			layout: []string{
				terramateConfig,
				`s:stacks/management/stack-a:after=["../stack-b"]`,
				`s:stacks/management/stack-b`,
				`s:stacks/management/stack-c:after=["../stack-a"]`,
				`s:stacks/network/stack-a:after=["../../operations/stack-a"]`,
				`s:stacks/network/stack-b`,
				`s:stacks/operations/stack-a:after=["../../management/stack-c"]`,
				`s:stacks/operations/stack-b`,
				`f:script.tm:
				script "drift" {
					description = "some description"
					job {
					  commands = [
						  ["echo", "cmd-1"],
						  ["echo", "cmd-2"],
					  ]
					}
				}`,
			},
			runScript: []string{"drift"},
			want: RunExpected{
				Stdout: `cmd-1cmd-2cmd-1cmd-2cmd-1cmd-2cmd-1cmd-2cmd-1cmd-2cmd-1cmd-2cmd-1cmd-2`,
				Stderr: "Script 0 at /script.tm:2,5-10,6 having 1 job(s)\n" +
					"/stacks/management/stack-b (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/management/stack-b (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/management/stack-a (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/management/stack-a (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/management/stack-c (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/management/stack-c (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/operations/stack-a (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/operations/stack-a (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/network/stack-a (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/network/stack-a (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/network/stack-b (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/network/stack-b (script:0 job:0.1)> echo cmd-2\n" +
					"/stacks/operations/stack-b (script:0 job:0.0)> echo cmd-1\n" +
					"/stacks/operations/stack-b (script:0 job:0.1)> echo cmd-2\n",
				FlattenStdout: true,
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sandboxes := []sandbox.S{
				sandbox.New(t),
			}

			for _, s := range sandboxes {
				s := s
				t.Run("run on sandbox", func(t *testing.T) {
					t.Parallel()
					s.BuildTree(tc.layout)

					wd := s.RootDir()
					if tc.workingDir != "" {
						wd = filepath.Join(wd, tc.workingDir)
					}

					// required because `terramate run-script` requires a clean repo.
					git := s.Git()
					git.CommitAll("everything")

					cli := NewCLI(t, wd)
					AssertRunResult(t, cli.RunScript(tc.runScript...), tc.want)
				})
			}
		})
	}
}

func TestRunScriptOnChangedStacks(t *testing.T) {
	t.Parallel()

	const (
		mainTfFileName  = "main.tf"
		mainTfContents  = "# change is the eternal truth of the universe"
		terramateConfig = `f:terramate.tm:
		  terramate {
			config {
			  experiments = ["scripts"]
			}
		  }`
	)

	s := sandbox.New(t)

	s.BuildTree([]string{
		terramateConfig,
		`s:stack`,
		`f:stack/globals.tm:
		globals {
		  message = "some message"
		}`,
		`f:stack/script.tm:
		  script "somescript" {
			description = "some description"
			job {
			  command = ["echo", "hello ${terramate.stack.name}"]
			}
		  }
		`,
		"s:stack/child",
	})

	cli := NewCLI(t, s.RootDir())
	git := s.Git()

	// after all files have been committed
	git.CommitAll("first commit")
	git.Push("main")

	// run-script should execute the script on both stacks
	AssertRunResult(t, cli.RunScript("--changed", "somescript"), RunExpected{
		Stdout: "hello stack\n" +
			"hello child\n",
		Stderr: "Script 0 at /stack/script.tm:2,5-7,6 having 1 job(s)\n" +
			"/stack (script:0 job:0.0)> echo hello stack\n" +
			"/stack/child (script:0 job:0.0)> echo hello child\n",
	})

	// when a stack is changed and committed
	stack := s.DirEntry("stack")
	stackMainTf := stack.CreateFile(mainTfFileName, "# some code")
	stackMainTf.Write(mainTfContents)
	git.CommitAll("second commit")
	git.Push("main")

	// run-script --changed should execute the script only on that stack
	AssertRunResult(t, cli.RunScript("--changed", "somescript"),
		RunExpected{
			Stdout: "hello stack\n",
			Stderr: "Script 0 at /stack/script.tm:2,5-7,6 having 1 job(s)\n" +
				"/stack (script:0 job:0.0)> echo hello stack\n",
		})

}
