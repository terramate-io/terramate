// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package printer

import (
	stderrors "errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
)

func ExampleNewPrinter() {
	p := NewPrinter(os.Stdout)
	p.Println("Doing something")
	p.Warnln("doing something fishy")
	p.ErrorWithDetailsln(
		"failed to find fish",
		stderrors.New("error 1"),
	)
	// Output:
	// Doing something
	// Warning: doing something fishy
	// Error: failed to find fish
	// > error 1
}

func TestPrinter(t *testing.T) {
	buf := new(strings.Builder)
	p := NewPrinter(buf)
	p.Println("Start to do something")
	p.Println("doing 1")
	p.Println("doing 2")
	p.Warnln("Something is not perfect, but continuing")
	p.Successln("Finished doing something")

	want := `Start to do something
doing 1
doing 2
Warning: Something is not perfect, but continuing
Finished doing something
`
	if got := buf.String(); got != want {
		t.Fatalf("want: %s, got: %s\n", want, got)
	}
}

func TestPrinterSimple(t *testing.T) {
	buf := new(strings.Builder)
	p := NewPrinter(buf)
	p.Println("Start to do something")
	p.ErrorWithDetailsln(
		"Wrong state of things",
		stderrors.New("details of the error here"))

	want := `Start to do something
Error: Wrong state of things
> details of the error here
`
	if got := buf.String(); got != want {
		t.Fatalf("want: %s, got: %s\n", want, got)
	}
}

func TestPrinterErrorWithDetails(t *testing.T) {
	type testcase struct {
		name string
		msg  string
		err  error
		want string
	}

	for _, tc := range []testcase{
		{
			name: "simple error",
			msg:  "Wrong state of things",
			err:  fmt.Errorf("error details"),
			want: `Error: Wrong state of things
> error details
`,
		},
		{
			name: "error with type *errors.Error",
			msg:  "Wrong state of things",
			err:  errors.E("some details"),
			want: `Error: Wrong state of things
> some details
`,
		},
		{
			name: "error of type *errors.List",
			msg:  "Wrong state of things",
			err: errors.L(
				errors.E("1. error details"),
				errors.E("2. error details"),
			),
			want: `Error: Wrong state of things
> 1. error details
> 2. error details
`,
		},
		{
			name: "error of type *errors.List with file ranges",
			msg:  "Parsing failed",
			err: errors.L(
				errors.E(errors.Kind("schema error"), hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				}, "unexpected attribute"),
				errors.E(errors.Kind("schema error"), hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 2, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 2, Column: 10, Byte: 13},
				}, "unexpected block"),
			),
			want: `Error: Parsing failed
> test.tm:1,5-10: schema error: unexpected attribute
> test.tm:2,5-10: schema error: unexpected block
`,
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			buf := new(strings.Builder)
			p := NewPrinter(buf)
			p.ErrorWithDetailsln(tc.msg, tc.err)

			got := buf.String()
			if tc.want != got {
				t.Errorf("unexpected result, want: %s, got %s", tc.want, got)
			}
		})
	}
}
