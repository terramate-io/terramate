// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate_test

import (
	"fmt"
	"testing"

	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
	"github.com/terramate-io/terramate/tf"
)

func TestGenerateVendor(t *testing.T) {
	t.Parallel()

	testCodeGeneration(t, []testcase{
		{
			name: "tm_vendor path is relative to generate blocks label",
			layout: []string{
				"s:stacks/stack",
				"s:stacks/stack/substack",
			},
			vendorDir: "/vendor",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v2")`),
						),
						GenerateHCL(
							Labels("dir/file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v3")`),
							),
						),
						GenerateFile(
							Labels("dir/file.txt"),
							Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v4")`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stacks/stack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../../../vendor/github.com/terramate-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../../../vendor/github.com/terramate-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "../../vendor/github.com/terramate-io/terramate/v1"),
						),
						"file.txt": stringer("../../vendor/github.com/terramate-io/terramate/v2"),
					},
				},
				{
					dir: "/stacks/stack/substack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../../../../vendor/github.com/terramate-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../../../../vendor/github.com/terramate-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "../../../vendor/github.com/terramate-io/terramate/v1"),
						),
						"file.txt": stringer("../../../vendor/github.com/terramate-io/terramate/v2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: project.NewPath("/stacks/stack"),
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
					{
						Dir: project.NewPath("/stacks/stack/substack"),
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
				},
			},
		},
		{
			name: "tm_vendor on root stack",
			layout: []string{
				"s:/",
				"s:substack",
			},
			vendorDir: "/vendor",
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v2")`),
						),
						GenerateHCL(
							Labels("dir/file.hcl"),
							Content(
								Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v3")`),
							),
						),
						GenerateFile(
							Labels("dir/file.txt"),
							Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v4")`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../vendor/github.com/terramate-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../vendor/github.com/terramate-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "vendor/github.com/terramate-io/terramate/v1"),
						),
						"file.txt": stringer("vendor/github.com/terramate-io/terramate/v2"),
					},
				},
				{
					dir: "/substack",
					files: map[string]fmt.Stringer{
						"dir/file.hcl": Doc(
							Str("vendor", "../../vendor/github.com/terramate-io/terramate/v3"),
						),
						"dir/file.txt": stringer("../../vendor/github.com/terramate-io/terramate/v4"),
						"file.hcl": Doc(
							Str("vendor", "../vendor/github.com/terramate-io/terramate/v1"),
						),
						"file.txt": stringer("../vendor/github.com/terramate-io/terramate/v2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: project.NewPath("/"),
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
					{
						Dir: project.NewPath("/substack"),
						Created: []string{
							"dir/file.hcl",
							"dir/file.txt",
							"file.hcl",
							"file.txt",
						},
					},
				},
			},
		},
		{
			name: "tm_vendor inside lets block",
			layout: []string{
				"s:stack",
			},
			vendorDir: "/vendor",
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateHCL(
							Labels("file.hcl"),
							Lets(
								Expr("source", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
							),
							Content(
								Expr("vendor", `let.source`),
							),
						),
						GenerateFile(
							Labels("file.txt"),
							Lets(
								Expr("source", `tm_vendor("github.com/terramate-io/terramate?ref=v2")`),
							),
							Expr("content", `let.source`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"file.hcl": Doc(
							Str("vendor", "../vendor/github.com/terramate-io/terramate/v1"),
						),
						"file.txt": stringer("../vendor/github.com/terramate-io/terramate/v2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir: project.NewPath("/stack"),
						Created: []string{
							"file.hcl",
							"file.txt",
						},
					},
				},
			},
		},
	})
}

func TestGenerateVendorRequestEvents(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{"s:stack"})
	rootentry := s.RootEntry()

	rootentry.CreateFile("config.tm", Doc(
		GenerateHCL(
			Labels("file.hcl"),
			Content(
				Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v1")`),
			),
		),
		GenerateFile(
			Labels("file.txt"),
			Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v2")`),
		),
		GenerateHCL(
			Labels("dir/file.hcl"),
			Content(
				Expr("vendor", `tm_vendor("github.com/terramate-io/terramate?ref=v3")`),
			),
		),
		GenerateFile(
			Labels("dir/file.txt"),
			Expr("content", `tm_vendor("github.com/terramate-io/terramate?ref=v4")`),
		),
	).String())

	src := func(source string) tf.Source {
		return test.ParseSource(t, source)
	}

	vendorDir := project.NewPath("/vendor")
	events := make(chan event.VendorRequest)
	gotEvents := []event.VendorRequest{}
	eventReceiverDone := make(chan struct{})

	go func() {
		for event := range events {
			gotEvents = append(gotEvents, event)
		}
		close(eventReceiverDone)
	}()

	t.Log("generating code")

	report := generate.Do(s.Config(), project.NewPath("/"), vendorDir, events)

	t.Logf("generation report: %s", report.Full())

	close(events)

	t.Log("waiting to receive all events")

	<-eventReceiverDone

	t.Log("received all events")

	wantEvents := []event.VendorRequest{
		{
			Source:    src("github.com/terramate-io/terramate?ref=v1"),
			VendorDir: vendorDir,
		},
		{
			Source:    src("github.com/terramate-io/terramate?ref=v2"),
			VendorDir: vendorDir,
		},
		{
			Source:    src("github.com/terramate-io/terramate?ref=v3"),
			VendorDir: vendorDir,
		},
		{
			Source:    src("github.com/terramate-io/terramate?ref=v4"),
			VendorDir: vendorDir,
		},
	}

	test.AssertEqualSets(t, gotEvents, wantEvents)
}
