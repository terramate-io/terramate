// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"

	"github.com/terramate-io/terramate/test/sandbox"
)

func TestScriptRun(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name       string
		layout     []string
		runScript  []string
		args       []string
		workingDir string
		env        []string
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
				Stdout: "hello" + "\n" +
					"some message" + "\n",
			},
		},
		{
			name: "script with lets referencing globals, env and terramate.* variables",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/globals.tm:
				globals {
				  msg = "some message"
				}`,
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  lets {
					msg = "run with global: ${global.msg}, terramate stack path: ${terramate.stack.path.absolute} and env: ${env.SOME_ENV}"
				  }
				  job {
					command = ["echo", "hello"]
				  }
				  job {
					command = ["echo", "${let.msg}"]
				  }
				}`,
				"s:stack-b",
			},
			env: []string{
				"SOME_ENV=SOME_ENV_VALUE",
			},
			runScript: []string{"somescript"},
			want: RunExpected{
				StderrRegexes: []string{
					"Script 0 at /stack-a/script.tm:.* having 2 job\\(s\\)",
					"/stack-a \\(script:0 job:0.0\\)> echo hello",
					"/stack-a \\(script:0 job:1.0\\)> echo run with global: some message, terramate stack path: /stack-a and env: SOME_ENV_VALUE",
				},
				Stdout: nljoin(
					"hello",
					"run with global: some message, terramate stack path: /stack-a and env: SOME_ENV_VALUE",
				),
			},
		},
		{
			name: "script with --reverse",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello1"]
				  }
				  job {
					command = ["echo", "hello2"]
				  }
				}`,
				"s:stack-a/stack-b",
			},
			runScript: []string{"somescript"},
			args:      []string{"--reverse"},
			want: RunExpected{
				Stderr: `Script 0 at /stack-a/script.tm:2,5-10,6 having 2 job(s)` + "\n" +
					"/stack-a/stack-b (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a/stack-b (script:0 job:1.0)> echo hello2" + "\n" +
					"/stack-a (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a (script:0 job:1.0)> echo hello2" + "\n",
				Stdout: "hello1" + "\n" +
					"hello2" + "\n" +
					"hello1" + "\n" +
					"hello2" + "\n",
			},
		},
		{
			name: "script with continue-on-error and all successful commands",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello1"]
				  }
				  job {
					command = ["echo", "hello2"]
				  }
				}`,
				"s:stack-a/stack-b",
			},
			runScript: []string{"somescript"},
			args:      []string{"--continue-on-error"},
			want: RunExpected{
				Stderr: `Script 0 at /stack-a/script.tm:2,5-10,6 having 2 job(s)` + "\n" +
					"/stack-a (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a (script:0 job:1.0)> echo hello2" + "\n" +
					"/stack-a/stack-b (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a/stack-b (script:0 job:1.0)> echo hello2" + "\n",
				Stdout: "hello1" + "\n" +
					"hello2" + "\n" +
					"hello1" + "\n" +
					"hello2" + "\n",
			},
		},
		{
			name: "script with continue-on-error and an unknown command",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello1"]
				  }
				  job {
					command = ["someunknowncommand"]
				  }
				  job {
					command = ["echo", "hello2"]
				  }
				}`,
				"s:stack-a/stack-b",
			},
			runScript: []string{"somescript"},
			args:      []string{"--continue-on-error"},
			want: RunExpected{
				Status: 1,
				Stderr: `Script 0 at /stack-a/script.tm:2,5-13,6 having 3 job(s)` + "\n" +
					"/stack-a (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a (script:0 job:1.0)> someunknowncommand" + "\n" +
					"/stack-a/stack-b (script:0 job:0.0)> echo hello1" + "\n" +
					"/stack-a/stack-b (script:0 job:1.0)> someunknowncommand" + "\n" +
					"Error: one or more commands failed" + "\n" +
					"> executable file not found in $PATH: running " + "`someunknowncommand`" + " in stack /stack-a: someunknowncommand" + "\n" +
					"> executable file not found in $PATH: running " + "`someunknowncommand`" + " in stack /stack-a/stack-b: someunknowncommand" + "\n",
				Stdout: "hello1" + "\n" +
					"hello1" + "\n",
			},
		},
		{
			name: "script with continue-on-error and a command that returns non-zero exit code",
			layout: []string{
				terramateConfig,
				"s:stack-a",
				`f:stack-a/script.tm:
				script "somescript" {
				  description = "some description"
				  job {
					command = ["echo", "hello1"]
				  }
				  job {
					command = ["` + HelperPath + `", "false"]
				  }
				  job {
					command = ["echo", "hello2"]
				  }
				}`,
				"s:stack-a/stack-b",
			},
			runScript: []string{"somescript"},
			args:      []string{"--continue-on-error"},
			want: RunExpected{
				Status: 1,
				Stderr: "Script 0 at /stack-a/script.tm:2,5-13,6 having 3 job(s)\n" +
					"/stack-a (script:0 job:0.0)> echo hello1\n" +
					"/stack-a (script:0 job:1.0)> " + HelperPath + " false\n" +
					"/stack-a/stack-b (script:0 job:0.0)> echo hello1\n" +
					"/stack-a/stack-b (script:0 job:1.0)> " + HelperPath + " false\n" +
					"Error: one or more commands failed\n" +
					"> execution failed: running " + HelperPath + " false (in /stack-a): exit status 1\n" +
					"> execution failed: running " + HelperPath + " false (in /stack-a/stack-b): exit status 1\n",
				Stdout: "hello1" + "\n" +
					"hello1" + "\n",
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

					env := RemoveEnv(os.Environ(), "CI", "GITHUB_ACTIONS", "GITHUB_TOKEN")
					env = append(env, tc.env...)

					cli := NewCLI(t, wd, env...)
					args := tc.args
					args = append(args, tc.runScript...)
					AssertRunResult(t, cli.RunScript(args...), tc.want)
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
