// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package execgrpc

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliconfig"
)

type testCLI struct {
	stdin  io.Reader
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (c *testCLI) Version() string       { return "test" }
func (c *testCLI) Product() string       { return "terramate" }
func (c *testCLI) PrettyProduct() string { return "Terramate" }
func (c *testCLI) WorkingDir() string    { return "" }
func (c *testCLI) Printers() printer.Printers {
	return printer.Printers{Stdout: printer.NewPrinter(c.stdout), Stderr: printer.NewPrinter(c.stderr)}
}
func (c *testCLI) Stdout() io.Writer            { return c.stdout }
func (c *testCLI) Stderr() io.Writer            { return c.stderr }
func (c *testCLI) Stdin() io.Reader             { return c.stdin }
func (c *testCLI) Config() cliconfig.Config     { return cliconfig.Config{} }
func (c *testCLI) Engine() *engine.Engine       { return nil }
func (c *testCLI) Reload(context.Context) error { return nil }
func (c *testCLI) ShowForm(context.Context, *pb.FormRequest) (*pb.FormResponse, error) {
	return nil, errors.E("form service not supported in test CLI")
}

func TestRenderFormTextInputDefault(t *testing.T) {
	t.Setenv("TM_FORM_AUTOFILL_DEFAULTS", "1")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cli := &testCLI{
		stdin:  bytes.NewBufferString("\n"),
		stdout: stdout,
		stderr: stderr,
	}

	resp, err := renderForm(cli, &pb.FormRequest{
		Id:    "form",
		Title: "Test",
		Fields: []*pb.FormField{
			{
				Id:           "name",
				Title:        "Name",
				Required:     true,
				DefaultValue: "hello",
				FieldType: &pb.FormField_TextInput{
					TextInput: &pb.TextInputFormField{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render form: %v", err)
	}
	if got := resp.Values["name"]; got != "hello" {
		t.Fatalf("expected default value to be returned, got %q", got)
	}
}
