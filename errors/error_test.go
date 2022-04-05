package errors_test

import (
	stderrors "errors"
	stdfmt "fmt"
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
)

var E = errors.E

func TestNoArgs(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("E() did not panic")
		}
	}()
	_ = E()
}

func TestErrorString(t *testing.T) {
	type testcase struct {
		name string
		args []interface{}
		want string
	}

	for _, tc := range []testcase{
		{
			name: "simple message",
			args: []interface{}{
				"error",
			},
			want: "error",
		},
		{
			name: "simple message with kind",
			args: []interface{}{
				errors.HCLSyntax,
				"failed to parse config",
			},
			want: fmt("%s: failed to parse config", errors.HCLSyntax),
		},
		{
			name: "the kind of previous error is promoted if new one lacks it",
			args: []interface{}{
				"failed to parse config",
				E(errors.HCLSyntax, "unexpected IDENT"),
			},
			want: fmt("%s: failed to parse config: unexpected IDENT", errors.HCLSyntax),
		},
		{
			name: "multiple different error kinds",
			args: []interface{}{
				errors.TerramateSchema, "failed to parse config",
				E(errors.TerraformSchema, "malformed terraform code"),
			},
			want: fmt(
				"%s: failed to parse config: %s: malformed terraform code",
				errors.TerramateSchema, errors.TerraformSchema,
			),
		},
		{
			name: "the file range gets promoted if current error lacks the file context",
			args: []interface{}{
				"failed to parse config",
				E(errors.TerramateSchema, "unexpected attribute name", hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				}),
			},
			want: fmt("test.tm:1,5-10: %s: failed to parse config: unexpected attribute name",
				errors.TerramateSchema),
		},
		{
			name: "nested errors",
			args: []interface{}{
				"1", E("2", E("3")),
			},
			want: "1: 2: 3",
		},
		{
			name: "nested errors with last one with range",
			args: []interface{}{
				"1", E("2", E("3", hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				})),
			},
			want: "test.tm:1,5-10: 1: 2: 3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := errors.E(tc.args...)
			assert.EqualStrings(t, tc.want, e.Error())
		})
	}
}

func TestErrorIs(t *testing.T) {
	type testcase struct {
		name    string
		args    []interface{}
		target  error
		areSame bool
	}

	for _, tc := range []testcase{
		{
			name:   "nil target is not equal",
			args:   []interface{}{"any error"},
			target: nil,
		},
		{
			name:   "*error.Error with no underlying error is not comparable with stderrors",
			args:   []interface{}{"any error"},
			target: stderrors.New("any error"),
		},
		{
			name:   "kind Any are never comparable",
			args:   []interface{}{"any error"},
			target: E("any error"),
		},
		{
			name:    "same kind",
			args:    []interface{}{errors.HCLSyntax, "error"},
			target:  E(errors.HCLSyntax),
			areSame: true,
		},
		{
			name:    "same underlying kind",
			args:    []interface{}{"error", E(errors.HCLSyntax)},
			target:  E(errors.HCLSyntax),
			areSame: true,
		},
		{
			name:    "same underlying kind (deep nested)",
			args:    []interface{}{"error", E(errors.TerraformSchema, E(errors.TerramateSchema, E(errors.HCLSyntax)))},
			target:  E(errors.HCLSyntax),
			areSame: true,
		},
		{
			name:    "underlying error is of the provided stderror",
			args:    []interface{}{"some error wrapping a stderror", os.ErrNotExist},
			target:  os.ErrNotExist,
			areSame: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := E(tc.args...)
			if errors.Is(e, tc.target) != tc.areSame {
				t.Fatalf("error[%v] is not target[%v]", e, tc.target)
			}
		})
	}
}

var fmt = stdfmt.Sprintf
