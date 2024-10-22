// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
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
					StderrRegex:   regexp.QuoteMeta("Warning: failed to execute `sharing_backend` command: exit status 1"),
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
