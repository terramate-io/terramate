package render_test

import (
	stderrors "errors"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli/render"
	"github.com/terramate-io/terramate/errors"
)

const (
	syntaxError   errors.Kind = "syntax error"
	tmSchemaError errors.Kind = "terramate schema error"
)

func ExampleNewText() {
	text := render.NewText(os.Stdout)
	text.Println("Doing something")
	text.Warnln("doing something fishy")
	text.ErrorWithDetailsln(
		"failed to find fish",
		errors.L(
			stderrors.New("error 1"),
			stderrors.New("error 2"),
		))
	// Output:
	// Doing something
	// Warning: doing something fishy
	// Error: failed to find fish
	// -> error 1
	// -> error 2
}

func TestTextSuccess(t *testing.T) {
	buf := new(strings.Builder)
	text := render.NewText(buf)
	text.Println("Start to do something")
	text.Println("doing 1")
	text.Println("doing 2")
	text.Warnln("Something is not perfect, but continuing")
	text.Successln("Finished doing something")

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

func TestTextError(t *testing.T) {
	buf := new(strings.Builder)
	text := render.NewText(buf)
	text.Println("Start to do something")
	text.Errorln("Wrong state of things")
	text.ErrorDetailsln(stderrors.New("details of the error here"))

	want := `Start to do something
Error: Wrong state of things
-> details of the error here
`
	if got := buf.String(); got != want {
		t.Fatalf("want: %s, got: %s\n", want, got)
	}
}

func TestTextErrorWithDetails(t *testing.T) {
	type testcase struct {
		name string
		msg  string
		err  error
		want string
	}

	for _, tc := range []testcase{
		{
			name: "list of errors",
			msg:  "Wrong state of things",
			err: errors.E(
				errors.Kind("syntax error"),
				errors.L(
					stderrors.New("err1"),
					stderrors.New("err2"),
				),
			),
			want: `Error: Wrong state of things
-> syntax error: err1
-> syntax error: err2
`,
		},
		{
			name: "error with range",
			msg:  "Wrong state of things",
			err: errors.E(
				tmSchemaError,
				hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				},
				"unexpected attribute name",
			),
			want: `Error: Wrong state of things
-> test.tm:1,5-10: terramate schema error: unexpected attribute name
`,
		},
		{
			name: "list of errors with range",
			msg:  "Wrong state of things",
			err: errors.L(
				errors.E(
					tmSchemaError,
					hcl.Range{
						Filename: "test.tm",
						Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
					"unexpected attribute name",
				),
				stderrors.New("some stdlib error"),
			),
			want: `Error: Wrong state of things
-> test.tm:1,5-10: terramate schema error: unexpected attribute name
-> some stdlib error
`,
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			buf := new(strings.Builder)
			text := render.NewText(buf)
			text.ErrorWithDetailsln(tc.msg, tc.err)

			got := buf.String()
			assert.EqualStrings(t, tc.want, got)
		})
	}
}
