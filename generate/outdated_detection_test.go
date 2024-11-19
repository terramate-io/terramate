// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test/sandbox"

	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

func TestOutdatedDetection(t *testing.T) {
	t.Parallel()
	type (
		file struct {
			path string
			body fmt.Stringer
		}
		step struct {
			wd        string
			layout    []string
			vendorDir string
			files     []file
			want      []string
			wantErr   error
		}
		testcase struct {
			name  string
			steps []step
		}
	)

	tcases := []testcase{
		{
			name: "empty project",
			steps: []step{
				{
					want: []string{},
				},
			},
		},
		{
			name: "project with no stacks",
			steps: []step{
				{
					layout: []string{
						"d:emptydir",
						"f:dir/file",
						"f:dir2/file",
					},
					want: []string{},
				},
			},
		},
		{
			name: "generate blocks with no code generated and one stack",
			steps: []step{
				{
					layout: []string{
						"s:stack",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack/test.hcl",
						"stack/test.txt",
					},
				},
			},
		},
		{
			name: "multiple stacks generating code",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "failing assertions outside generate blocks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								Assert(
									Bool("assertion", false),
									Str("message", "msg"),
								),
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					wantErr: errors.L(
						errors.E(generate.ErrAssertion),
						errors.E(generate.ErrAssertion),
					),
				},
			},
		},
		{
			name: "failing assertion inside generate_hcl block",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Assert(
										Bool("assertion", false),
										Str("message", "msg"),
									),
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					wantErr: errors.L(
						errors.E(generate.ErrAssertion),
						errors.E(generate.ErrAssertion),
					),
				},
			},
		},
		{
			name: "failing assertion inside generate_file block",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Assert(
										Bool("assertion", false),
										Str("message", "msg"),
									),
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					wantErr: errors.L(
						errors.E(generate.ErrAssertion),
						errors.E(generate.ErrAssertion),
					),
				},
			},
		},
		{
			name: "warning assertions outside generate blocks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								Assert(
									Bool("assertion", false),
									Bool("warning", true),
									Str("message", "msg"),
								),
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "warning assertions inside generate blocks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Assert(
										Bool("assertion", false),
										Bool("warning", true),
										Str("message", "msg"),
									),
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Assert(
										Bool("assertion", false),
										Bool("warning", true),
										Str("message", "msg"),
									),
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "generate_file block content changed",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "changed"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "changed"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "changed"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "generate_hcl/generate_file deletes on deleted stack",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "stack-2/" + stack.DefaultFilename,
							body: Doc(),
						},
					},
					want: []string{
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "generate_file.context=root detects changed config",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("/test.txt"),
									Expr("context", `root`),
									Str("content", "tm is awesome"),
								),
							),
						},
					},
					want: []string{
						"test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("/test.txt"),
									Expr("context", `root`),
									Str("content", "tm has changed"),
								),
							),
						},
					},
					want: []string{
						"test.txt",
					},
				},
			},
		},
		{
			name: "generate_file.context=root detects on manually changed file",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("/test.txt"),
									Expr("context", `root`),
									Str("content", "tm is awesome"),
								),
							),
						},
					},
					want: []string{
						"test.txt",
					},
				},
				{
					files: []file{
						{
							path: "test.txt",
							body: stringer("whatever"),
						},
					},
					want: []string{
						"test.txt",
					},
				},
			},
		},
		{
			name: "detecting outdated code on root",
			steps: []step{
				{
					layout: []string{
						"s:/",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"test.hcl",
						"test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"test.hcl",
						"test.txt",
					},
				},
			},
		},
		{
			name: "detecting outdated code on root with sub-stacks",
			steps: []step{
				{
					layout: []string{
						"s:/",
						"s:/stack-1",
						"s:/stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
						"test.hcl",
						"test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
						"test.hcl",
						"test.txt",
					},
				},
			},
		},
		{
			name: "detecting orphan files from root dir",
			steps: []step{
				{
					layout: []string{
						"s:/",
						"s:/stack-1",
						"s:/stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
						"test.hcl",
						"test.txt",
					},
				},
				{
					files: []file{
						{
							path: "stack.tm.hcl",
							body: Doc(),
						},
					},
					want: []string{
						"test.hcl",
					},
				},
			},
		},
		{
			name: "running from a subdir of a stack do not misdetect as orphan files",
			steps: []step{
				{
					layout: []string{
						"s:/",
						"s:/stack-1",
						"s:/stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("subdir/test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("subdir/test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/subdir/test.hcl",
						"stack-1/subdir/test.txt",
						"stack-2/subdir/test.hcl",
						"stack-2/subdir/test.txt",
						"subdir/test.hcl",
						"subdir/test.txt",
					},
				},
				{
					// this step just executes in the subdir to catch outdated code (if any)
					wd: "/subdir",
				},
			},
		},
		{
			name: "running from a subdir still detects orphan files",
			steps: []step{
				{
					layout: []string{
						"s:/",
						"s:/stack-1",
						"s:/stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("subdir/test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("subdir/test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/subdir/test.hcl",
						"stack-1/subdir/test.txt",
						"stack-2/subdir/test.hcl",
						"stack-2/subdir/test.txt",
						"subdir/test.hcl",
						"subdir/test.txt",
					},
				},
				{
					// this step just executes in the subdir to catch outdated code (if any)
					wd: "/subdir",
					files: []file{
						{
							path: "/stack.tm.hcl",
							body: Doc(),
						},
					},
					want: []string{
						"subdir/test.hcl",
					},
				},
			},
		},
		{
			name: "changing vendorDir changes all generate blocks calling tm_vendor",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					vendorDir: "/vendor",
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("vendor.txt"),
									Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
								),
								GenerateFile(
									Labels("file.txt"),
									Str("content", "something"),
								),
								GenerateHCL(
									Labels("vendor.hcl"),
									Content(
										Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
									),
								),
								GenerateHCL(
									Labels("file.hcl"),
									Content(
										Str("content", "something"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/file.hcl",
						"stack-1/file.txt",
						"stack-1/vendor.hcl",
						"stack-1/vendor.txt",
						"stack-2/file.hcl",
						"stack-2/file.txt",
						"stack-2/vendor.hcl",
						"stack-2/vendor.txt",
					},
				},
				{
					vendorDir: "/modules",
					want: []string{
						"stack-1/vendor.hcl",
						"stack-1/vendor.txt",
						"stack-2/vendor.hcl",
						"stack-2/vendor.txt",
					},
				},
			},
		},
		{
			name: "moving generate blocks to different files is not detected on generate_hcl",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
						{
							path: "generate_file.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
							),
						},
						{
							path: "generate_hcl.tm",
							body: Doc(
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{},
				},
			},
		},
		{
			name: "generate_file is not detected when deleted",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "tm is awesome"),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
					},
					want: []string{},
				},
			},
		},
		{
			name: "generate_hcl is detected when deleted",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
			},
		},
		{
			name: "tmgen is detected when changed",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
					},
					files: []file{
						{
							path: "terramate.tm",
							body: Terramate(
								Config(
									Expr("experiments", `["tmgen"]`),
								),
							),
						},
						{
							path: "stack-1/test.hcl.tmgen",
							body: Doc(
								Str("content", "tmgen"),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "stack-1/test.hcl.tmgen",
							body: Doc(),
						},
					},
					want: []string{
						"stack-1/test.hcl",
					},
				},
			},
		},
		{
			name: "generate blocks shifting condition",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.txt",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-2/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
			},
		},
		{
			name: "multiple generate blocks with same label",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code1"),
								),
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "code2"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code1"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "code3"),
									),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
							),
						},
					},
					want: []string{},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", true),
									Str("content", "code3"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", true),
									Content(
										Str("content", "code2"),
									),
								),
							),
						},
					},
					want: []string{},
				},
			},
		},
		{
			name: "ignores outdated code on skipped dir",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-2",
						"f:stack-2/" + terramate.SkipFilename,
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Str("content", "code"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/test.hcl",
						"stack-1/test.txt",
					},
				},
			},
		},
		{
			name: "detection on substacks",
			steps: []step{
				{
					layout: []string{
						"s:stack-1",
						"s:stack-1/child",
						"s:stack-2",
						"s:stack-2/dir/child",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								Globals(
									Bool("condition", true),
								),
								GenerateFile(
									Labels("test.txt"),
									Expr("condition", "global.condition"),
									Str("content", "code"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Expr("condition", "global.condition"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"stack-1/child/test.hcl",
						"stack-1/child/test.txt",
						"stack-1/test.hcl",
						"stack-1/test.txt",
						"stack-2/dir/child/test.hcl",
						"stack-2/dir/child/test.txt",
						"stack-2/test.hcl",
						"stack-2/test.txt",
					},
				},
				{
					files: []file{
						{
							path: "stack-1/child/config.tm",
							body: Doc(
								Globals(
									Bool("condition", false),
								),
							),
						},
						{
							path: "stack-2/dir/child/config.tm",
							body: Doc(
								Globals(
									Bool("condition", false),
								),
							),
						},
					},
					want: []string{
						"stack-1/child/test.hcl",
						"stack-1/child/test.txt",
						"stack-2/dir/child/test.hcl",
						"stack-2/dir/child/test.txt",
					},
				},
			},
		},
		{
			name: "detecting outdated code when stack generates in subdir",
			steps: []step{
				{
					layout: []string{
						"s:/",
						"d:/subdir",
						"s:/stack-1",
						"s:/stack-2",
						"d:/stack-1/somedir",
						"d:/stack-2/somedir",
					},
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateHCL(
									Labels("somedir/test.hcl"),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"somedir/test.hcl",
						"stack-1/somedir/test.hcl",
						"stack-2/somedir/test.hcl",
					},
				},
				{
					files: []file{
						{
							path: "config.tm",
							body: Doc(
								GenerateFile(
									Labels("test.txt"),
									Bool("condition", false),
									Str("content", "tm is awesome"),
								),
								GenerateHCL(
									Labels("test.hcl"),
									Bool("condition", false),
									Content(
										Str("content", "tm is awesome"),
									),
								),
							),
						},
					},
					want: []string{
						"somedir/test.hcl",
						"stack-1/somedir/test.hcl",
						"stack-2/somedir/test.hcl",
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
			for i, step := range tcase.steps {
				t.Logf("step %d", i)

				s.BuildTree(step.layout)
				root := s.RootEntry()

				for _, file := range step.files {
					root.CreateFile(file.path, file.body.String())
				}

				s.ReloadConfig()

				vendorDir := project.NewPath("/modules")
				if step.vendorDir != "" {
					vendorDir = project.NewPath(step.vendorDir)
				}

				target := s.Config().Tree()
				if step.wd != "" {
					var ok bool
					target, ok = s.Config().Lookup(project.NewPath(step.wd))
					if !ok {
						panic("invalid step.wd working directory")
					}
				}

				got, err := generate.DetectOutdated(s.Config(), target, vendorDir)
				assert.IsError(t, err, step.wantErr)
				if err != nil {
					continue
				}

				assertEqualStringList(t, got, step.want)

				t.Log("checking that after generate outdated detection should always return empty")

				s.GenerateWith(s.Config(), vendorDir)
				got, err = generate.DetectOutdated(s.Config(), s.Config().Tree(), vendorDir)
				assert.NoError(t, err)

				assertEqualStringList(t, got, []string{})
			}
		})
	}
}
