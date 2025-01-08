// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/hcl"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestRunSharing(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name         string
		layout       []string
		extraRunArgs []string
		check        func(t *testing.T, s *sandbox.S, res RunResult)
	}

	for _, tc := range []testcase{
		{
			name: "basic sharing - 1 output and 1 input",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, s *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					IgnoreStdout: true,
				})
				const expected = "s1_content"
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s1/file.txt")))
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s2/file.txt")))
			},
		},
		{
			name: "parent/child sharing",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Output(
					Labels("s1_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s1_file.content"),
				).String(),
				"s:s1/s2",
				"f:s1/s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Expr("from_stack_id", `terramate.stack.parent.id`),
				).String(),
				"f:s1/s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, s *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					IgnoreStdout: true,
				})
				const expected = "s1_content"
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s1/file.txt")))
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s1/s2/file.txt")))
			},
		},
		{
			name: "input with no output counterpart",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, _ *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					Status:       1,
					IgnoreStdout: true,
					StderrRegex:  `This object does not have an attribute named "s1_output"`,
				})
			},
		},
		{
			name: "stacks needs to be ordered manually",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s2:id=s2",
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Str("content", "s2_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"f:s2/output.tm:" + Output(
					Labels("s2_output"),
					Str("backend", "name"),
					Expr("value", "resource.local_file.s2_file.content"),
				).String(),
				`s:s1:after=["/s2"]`,
				"f:s1/input.tm:" + Input(
					Labels("s1_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s2_output.value"),
					Str("from_stack_id", "s2"),
				).String(),
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Expr("content", "var.s1_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, s *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					IgnoreStdout: true,
				})
				const expected = "s2_content"
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s1/file.txt")))
				assert.EqualStrings(t, expected, string(s.RootEntry().ReadFile("s2/file.txt")))
			},
		},
		{
			name: "mocking unknown values",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
					Str("mock", "mocked value"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", "var.s2_input"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, s *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					IgnoreStdout: true,
				})
				assert.EqualStrings(t, "s1_content", string(s.RootEntry().ReadFile("s1/file.txt")))
				assert.EqualStrings(t, "mocked value", string(s.RootEntry().ReadFile("s2/file.txt")))
			},
		},
		{
			name: "multiple outputs",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command("terraform", "output", "-json"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file1"),
						Str("content", "s1_content1"),
						Str("filename", "${path.module}/file1.txt"),
					),
					Block("resource",
						Labels("local_file", "s1_file2"),
						Str("content", "s1_content2"),
						Str("filename", "${path.module}/file2.txt"),
					),
				).String(),
				"f:s1/output.tm:" + Doc(
					Output(
						Labels("s1_output1"),
						Str("backend", "name"),
						Expr("value", "resource.local_file.s1_file1.content"),
					),
					Output(
						Labels("s1_output2"),
						Str("backend", "name"),
						Expr("value", "resource.local_file.s1_file2.content"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Doc(
					Input(
						Labels("s2_input1"),
						Str("backend", "name"),
						Expr("value", "outputs.s1_output1.value"),
						Str("from_stack_id", "s1"),
					),
					Input(
						Labels("s2_input2"),
						Str("backend", "name"),
						Expr("value", "outputs.s1_output2.value"),
						Str("from_stack_id", "s1"),
					),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Expr("content", `"${var.s2_input1} - ${var.s2_input2}"`),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
			},
			check: func(t *testing.T, s *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					IgnoreStdout: true,
				})
				const s1expected1 = "s1_content1"
				const s1expected2 = "s1_content2"
				const s2expected = "s1_content1 - s1_content2"
				assert.EqualStrings(t, s1expected1, string(s.RootEntry().ReadFile("s1/file1.txt")))
				assert.EqualStrings(t, s1expected2, string(s.RootEntry().ReadFile("s1/file2.txt")))
				assert.EqualStrings(t, s2expected, string(s.RootEntry().ReadFile("s2/file.txt")))
			},
		},
		{
			name: "sharing config with --continue-on-error",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command(HelperPath, "exit", "1"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
				"f:s1/output.tm:" + Doc(
					Output(
						Labels("s1_output"),
						Str("backend", "name"),
						Expr("value", "resource.local_file.s1_file.content"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Str("content", "not using output"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"s:s3:id=s3",
				"f:s3/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s3_file"),
						Str("content", "s3_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
			},
			extraRunArgs: []string{"--continue-on-error"},
			check: func(t *testing.T, _ *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						"Terraform will perform the following actions",
						"local_file.s1_file",
						"local_file.s3_file",
					},
					NoStdoutRegex: "local_file.s2_file",
					StderrRegexes: []string{
						regexp.QuoteMeta("Warning: failed to execute `sharing_backend`"),
						regexp.QuoteMeta("helper exit 1) (stdout: ) (stderr: ): exit status 1"),
					},
				})
			},
		},
		{
			name: "sharing config with --continue-on-error and from_stack_id not found",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command(HelperPathAsHCL, "echo", "{}"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
				"f:s1/output.tm:" + Doc(
					Output(
						Labels("s1_output"),
						Str("backend", "name"),
						Expr("value", "resource.local_file.s1_file.content"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "not-exists"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Str("content", "not using output"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"s:s3:id=s3",
				"f:s3/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s3_file"),
						Str("content", "s3_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
			},
			extraRunArgs: []string{"--continue-on-error"},
			check: func(t *testing.T, _ *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						"Terraform will perform the following actions",
						"local_file.s1_file",
						"local_file.s3_file",
					},
					StderrRegexes: []string{
						regexp.QuoteMeta(`Stack /s2 needs output from stack ID "not-exists" but it cannot be found`),
					},
					NoStdoutRegexes: []string{
						"local_file.s2_file will be created",
						`var.s2_input`,
						`Enter a value:`,
					},
				})
			},
		},
		{
			name: "sharing config with --continue-on-error and command do not return JSON",
			layout: []string{
				"f:backend.tm:" + Block("sharing_backend",
					Labels("name"),
					Expr("type", "terraform"),
					Str("filename", "sharing.tf"),
					Command(HelperPathAsHCL, "echo", "$error"),
				).String(),
				"s:s1:id=s1",
				"f:s1/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s1_file"),
						Str("content", "s1_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
				"f:s1/output.tm:" + Doc(
					Output(
						Labels("s1_output"),
						Str("backend", "name"),
						Expr("value", "resource.local_file.s1_file.content"),
					),
				).String(),
				"s:s2",
				"f:s2/input.tm:" + Input(
					Labels("s2_input"),
					Str("backend", "name"),
					Expr("value", "outputs.s1_output.value"),
					Str("from_stack_id", "s1"),
				).String(),
				"f:s2/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s2_file"),
						Str("content", "not using output"),
						Str("filename", "${path.module}/file.txt"),
					),
				).String(),
				"s:s3:id=s3",
				"f:s3/main.tf:" + Doc(
					Block("resource",
						Labels("local_file", "s3_file"),
						Str("content", "s3_content"),
						Str("filename", "${path.module}/foo.bar"),
					),
				).String(),
			},
			extraRunArgs: []string{"--continue-on-error"},
			check: func(t *testing.T, _ *sandbox.S, res RunResult) {
				AssertRunResult(t, res, RunExpected{
					Status: 1,
					StdoutRegexes: []string{
						"Terraform will perform the following actions",
						"local_file.s1_file",
						"local_file.s3_file",
					},
					StderrRegexes: []string{
						regexp.QuoteMeta(`unmashaling sharing_backend output`),
					},
					NoStdoutRegexes: []string{
						"local_file.s2_file will be created",
						`var.s2_input`,
						`Enter a value:`,
					},
				})
			},
		},
	} {
		tc := tc
		runCases := []bool{false, true}
		for _, isScript := range runCases {
			layout := append([]string{}, tc.layout...)
			name := tc.name
			if isScript {
				name += "/script"
			}
			t.Run(name, func(t *testing.T) {
				experiments := []string{hcl.SharingIsCaringExperimentName}
				if isScript {
					experiments = append(experiments, "scripts")

					layout = append(layout,
						`f:script.tm:`+Script(
							Labels("apply"),
							Str("name", "apply iac"),
							Block("job",
								Expr("command", `[
							"terraform", "apply", "-auto-approve", {
								enable_sharing = true
								mock_on_fail = true
							}
						]`),
							),
						).String(),
					)
				}
				layout = append(layout,
					"f:exp.tm:"+Terramate(
						Config(
							Experiments(experiments...),
						),
					).String(),
				)

				s := sandbox.New(t)
				s.BuildTree(layout)
				tmcli := NewCLI(t, s.RootDir())
				tmcli.PrependToPath(filepath.Dir(TerraformTestPath))
				res := tmcli.Run("run", HelperPath, "echo", "hello")
				if res.Status == 0 {
					// generate safeguard must trigger
					t.Fatal("run must fail if sharing is not generated")
				}
				AssertRunResult(t, tmcli.Run("generate"), RunExpected{
					IgnoreStdout: true,
				})
				AssertRunResult(t, tmcli.Run("run", "--quiet", "-X", "terraform", "init"),
					RunExpected{
						IgnoreStdout: true,
					},
				)
				s.Git().CommitAll("all")
				if isScript {
					args := []string{"script", "run", "--quiet"}
					args = append(args, tc.extraRunArgs...)
					args = append(args, "apply")
					res = tmcli.Run(args...)
				} else {
					args := []string{"run", "--quiet", "--enable-sharing", "--mock-on-fail"}
					args = append(args, tc.extraRunArgs...)
					args = append(args, "terraform", "apply", "-auto-approve")
					res = tmcli.Run(args...)
				}
				tc.check(t, &s, res)
			})
		}
	}
}

func TestRunSharingParallel(t *testing.T) {
	// This test triggers the race condition described in SC-14248.
	t.Parallel()

	layout := []string{
		"f:backend.tm:" + Block("sharing_backend",
			Labels("name"),
			Expr("type", "terraform"),
			Str("filename", "sharing.tf"),
			Command("terraform", "output", "-json"),
		).String(),
		"s:s1:id=s1",
		"f:s1/main.tf:" + Doc(
			Block("resource",
				Labels("local_file", "s1_file"),
				Str("content", "s1_content"),
				Str("filename", "${path.module}/file.txt"),
			),
		).String(),
		"f:s1/output.tm:" + Output(
			Labels("s1_output"),
			Str("backend", "name"),
			Expr("value", "resource.local_file.s1_file.content"),
		).String(),
	}

	for i := 2; i < 5; i++ {
		layout = append(layout,
			fmt.Sprintf(`s:s%d:after=["/s1"]`, i),
			fmt.Sprintf("f:s%d/input.tm:", i)+Input(
				Labels(fmt.Sprintf("s%d_input", i)),
				Str("backend", "name"),
				Expr("value", "outputs.s1_output.value"),
				Str("from_stack_id", "s1"),
			).String(),
			fmt.Sprintf("f:s%d/main.tf:", i)+Doc(
				Block("resource",
					Labels("local_file", fmt.Sprintf("s%d_file", i)),
					Expr("content", fmt.Sprintf("var.s%d_input", i)),
					Str("filename", "${path.module}/file.txt"),
				),
			).String(),
		)
	}

	layout = append(layout,
		"f:exp.tm:"+Terramate(
			Config(
				Experiments(hcl.SharingIsCaringExperimentName),
			),
		).String(),
	)

	s := sandbox.New(t)
	s.BuildTree(layout)

	tmcli := NewCLI(t, s.RootDir())
	tmcli.PrependToPath(filepath.Dir(TerraformTestPath))
	res := tmcli.Run("run", HelperPath, "echo", "hello")

	if res.Status == 0 {
		// generate safeguard must trigger
		t.Fatal("run must fail if sharing is not generated")
	}

	AssertRunResult(t, tmcli.Run("generate"), RunExpected{
		IgnoreStdout: true,
	})
	AssertRunResult(t, tmcli.Run("run", "--quiet", "-X", "terraform", "init"),
		RunExpected{
			IgnoreStdout: true,
		},
	)
	s.Git().CommitAll("all")

	AssertRunResult(t, tmcli.Run("run", "--quiet", "--enable-sharing", "--mock-on-fail", "--parallel=10", "terraform", "apply", "-auto-approve"),
		RunExpected{
			IgnoreStdout: true,
		},
	)
}

func TestRunOutputDependencies(t *testing.T) {
	t.Parallel()

	t.Run("--*-output-dependencies must show an error if experiment is disabled", func(t *testing.T) {
		t.Parallel()
		s := sandbox.New(t)
		// NOTE: outputs-sharing experiment is still not enabled by default.

		s.BuildTree([]string{
			`f:terramate.tm:` + Terramate(
				Config(
					Experiments("scripts"),
				),
			).String(),
			`f:script.tm:` + Script(
				Labels("test"),
				Block("job",
					Command(HelperPathAsHCL, "stack-abs-path", fmt.Sprintf(`${tm_chomp(<<-EOF
					%s
				EOF
				)}`, s.RootDir())),
				),
			).String(),
		})

		expected := RunExpected{
			Status:      1,
			StderrRegex: regexp.QuoteMeta("--include-output-dependencies requires the 'outputs-sharing' experiment enabled"),
		}
		tmcli := NewCLI(t, s.RootDir())
		AssertRunResult(t, tmcli.Run("run", "-X", "--include-output-dependencies", "--", HelperPath, "stack-abs-path", s.RootDir()), expected)
		AssertRunResult(t, tmcli.Run("script", "run", "-X", "--include-output-dependencies", "test"), expected)
		AssertRunResult(t, tmcli.Run("run", "-X", "--only-output-dependencies", "--", HelperPath, "stack-abs-path", s.RootDir()), expected)
		AssertRunResult(t, tmcli.Run("script", "run", "-X", "--only-output-dependencies", "test"), expected)
	})

	type fixture struct {
		sandbox        *sandbox.S
		dependencyPath string
		dependentPath  string
	}

	setupSandbox := func(t *testing.T) fixture {
		s := sandbox.New(t)
		const stackDependencyName = "stack-dependency"
		const stackDependentName = "stack-dependent"
		s.BuildTree([]string{
			`f:terramate.tm:` + Terramate(
				Config(
					Experiments(hcl.SharingIsCaringExperimentName, "scripts"),
				),
			).String(),
			`f:script.tm:` + Script(
				Labels("test"),
				Block("job",
					Expr("command", fmt.Sprintf(
						`["%s", "stack-abs-path", env.TM_TEST_BASEDIR]`, HelperPathAsHCL),
					),
				),
			).String(),
			`f:sharing.tm:` + Block("sharing_backend",
				Labels("default"),
				Expr("type", "terraform"),
				Command("terraform", "output", "-json"),
				Str("filename", "_sharing.tf"),
			).String(),
			"s:" + stackDependencyName + `:id=` + stackDependencyName,
			`s:` + stackDependentName + `:after=["/stack-dependency"];tags=["dependent"]`,
		})

		s.RootEntry().CreateFile(stackDependencyName+"/outputs.tm", Block("output",
			Labels("output1"),
			Str("backend", "default"),
			Expr("value", "some.value"),
		).String())

		s.RootEntry().CreateFile(stackDependentName+"/inputs.tm", Block("input",
			Labels("input1"),
			Str("backend", "default"),
			Expr("value", "outputs.output1.value"),
			Str("from_stack_id", stackDependencyName),
		).String())

		s.Generate()
		s.Git().CommitAll("initial commit")
		s.Git().Push("main")
		return fixture{
			sandbox:        &s,
			dependencyPath: "/" + stackDependencyName,
			dependentPath:  "/" + stackDependentName,
		}
	}

	t.Run("must not show the output dependencies by default", func(t *testing.T) {
		t.Parallel()
		f := setupSandbox(t)

		// without any filters should just execute the stacks in scope

		{
			expected := RunExpected{Stdout: nljoin(f.dependencyPath, f.dependentPath)}
			cli := NewCLI(t, f.sandbox.RootDir())
			cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}
			AssertRunResult(t, cli.Run("run", "--quiet", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), expected)
			AssertRunResult(t, cli.Run("script", "run", "--quiet", "test"), expected)
		}

		{
			for _, stack := range []string{f.dependencyPath, f.dependentPath} {
				expected := RunExpected{Stdout: nljoin(stack)}
				cli := NewCLI(t, filepath.Join(f.sandbox.RootDir(), stack))
				cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}
				AssertRunResult(t, cli.Run("run", "--quiet", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), expected)
				AssertRunResult(t, cli.Run("script", "run", "--quiet", "test"), expected)
			}
		}

		{
			// when filtering, no output dependencies should be shown (by default)

			git := f.sandbox.Git()
			git.CheckoutNew("change-stack-dependent")
			f.sandbox.DirEntry(f.dependentPath).CreateFile("main.tf", "# add file")

			expected := RunExpected{Stdout: nljoin(f.dependentPath)}

			cli := NewCLI(t, f.sandbox.RootDir())
			cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}
			AssertRunResult(t, cli.Run("run", "-X", "--quiet", "--changed", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), expected)
			AssertRunResult(t, cli.Run("run", "-X", "--quiet", "--tags=dependent", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), expected)
			AssertRunResult(t, cli.Run("script", "run", "-X", "--changed", "--quiet", "test"), expected)
			AssertRunResult(t, cli.Run("script", "run", "-X", "--tags=dependent", "--quiet", "test"), expected)
		}
	})

	t.Run("--*-output-dependencies pull dependencies", func(t *testing.T) {
		t.Parallel()
		f := setupSandbox(t)

		// if dependency also changed, they must return once (no duplicates).

		f.sandbox.Git().CheckoutNew("change-both-stacks")

		cli := NewCLI(t, f.sandbox.RootDir())
		f.sandbox.DirEntry(f.dependencyPath).CreateFile("main.tf", "# add file")
		f.sandbox.DirEntry(f.dependentPath).CreateFile("main.tf", "# add file")

		AssertRunResult(t, cli.Run("run", "--quiet", "--changed", "-X", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), RunExpected{
			Stdout: nljoin(f.dependencyPath, f.dependentPath),
		})

		f.sandbox.Git().CommitAll("change both")
		f.sandbox.Git().Checkout("main")
		f.sandbox.Git().Merge("change-both-stacks")
		f.sandbox.Git().Push("main")

		f.sandbox.Git().CheckoutNew("change-dependent")

		{
			// --*-output-dependencies must pull dependencies if they are out of scope
			// scope=changed

			cli := NewCLI(t, f.sandbox.RootDir())
			cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}

			normalExpected := RunExpected{Stdout: nljoin(f.dependentPath)}
			f.sandbox.DirEntry(f.dependentPath).CreateFile("main.tf", "# changed file")
			AssertRunResult(t, cli.Run("run", "-X", "--quiet", "--changed", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), normalExpected) // sanity check

			inclExpected := RunExpected{Stdout: nljoin(f.dependencyPath, f.dependentPath)}
			AssertRunResult(t, cli.Run("run", "--quiet", "-X", "--quiet", "--changed", "--include-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), inclExpected)
			AssertRunResult(t, cli.Run("script", "run", "--quiet", "-X", "--quiet", "--include-output-dependencies", "test"), inclExpected)
			onlyExpected := RunExpected{Stdout: nljoin(f.dependencyPath)}
			AssertRunResult(t, cli.Run("run", "--quiet", "-X", "--quiet", "--changed", "--only-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), onlyExpected)
			AssertRunResult(t, cli.Run("script", "run", "--quiet", "-X", "--quiet", "--only-output-dependencies", "test"), onlyExpected)
		}

		{
			// --*-output-dependencies must pull dependencies if they are out of scope
			// scope=path

			cwd := filepath.Join(f.sandbox.RootDir(), f.dependentPath)
			cli := NewCLI(t, cwd)
			cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}

			normalExpected := RunExpected{Stdout: nljoin(f.dependentPath)}
			f.sandbox.DirEntry(f.dependentPath).CreateFile("main.tf", "# changed file")
			AssertRunResult(t, cli.Run("run", "--quiet", "-X", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), normalExpected) // sanity check

			inclExpected := RunExpected{Stdout: nljoin(f.dependencyPath, f.dependentPath)}
			AssertRunResult(t, cli.Run("run", "--quiet", "-X", "--include-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), inclExpected)
			AssertRunResult(t, cli.Run("script", "run", "--quiet", "-X", "--include-output-dependencies", "test"), inclExpected)

			onlyExpected := RunExpected{Stdout: nljoin(f.dependencyPath)}
			AssertRunResult(t, cli.Run("run", "--quiet", "-X", "--only-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), onlyExpected)
			AssertRunResult(t, cli.Run("script", "run", "--quiet", "-X", "--only-output-dependencies", "test"), onlyExpected)
		}

		{
			// --*-output-dependencies must pull dependencies if they are out of scope
			// scope=tags

			cli := NewCLI(t, f.sandbox.RootDir())
			cli.AppendEnv = []string{"TM_TEST_BASEDIR=" + f.sandbox.RootDir()}

			normalExpected := RunExpected{Stdout: nljoin(f.dependentPath)}
			f.sandbox.DirEntry(f.dependentPath).CreateFile("main.tf", "# changed file")
			AssertRunResult(t, cli.Run("run", "--tags=dependent", "--quiet", "-X", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), normalExpected) // sanity check

			inclExpected := RunExpected{Stdout: nljoin(f.dependencyPath, f.dependentPath)}
			AssertRunResult(t, cli.Run("run", "--tags=dependent", "--quiet", "-X", "--include-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), inclExpected)
			AssertRunResult(t, cli.Run("script", "run", "--tags=dependent", "--quiet", "-X", "--include-output-dependencies", "test"), inclExpected)

			onlyExpected := RunExpected{Stdout: nljoin(f.dependencyPath)}
			AssertRunResult(t, cli.Run("run", "--tags=dependent", "--quiet", "-X", "--only-output-dependencies", "--", HelperPath, "stack-abs-path", f.sandbox.RootDir()), onlyExpected)
			AssertRunResult(t, cli.Run("script", "run", "--tags=dependent", "--quiet", "-X", "--only-output-dependencies", "test"), onlyExpected)

		}
	})
}
