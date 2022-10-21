// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

const (
	syntaxError   errors.Kind = "syntax error"
	tmSchemaError errors.Kind = "terramate schema error"
	tfSchemaError errors.Kind = "terraform schema error"
)

func TestNoArgs(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("E() did not panic")
		}
	}()
	_ = E()
}

func TestEmptyTopLevelError(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("E() did not panic")
		}
	}()
	_ = E(stderrors.New("test"))
}

func TestUnknownTypesWithNoFormat(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("E() did not panic")
		}
	}()
	_ = E(10, true, 2.5)
}

func TestErrorString(t *testing.T) {
	type testcase struct {
		name string
		err  error
		want string
	}

	var nildiagnostic *hcl.Diagnostic

	for _, tc := range []testcase{
		{
			name: "simple message",
			err:  E("error"),
			want: "error",
		},
		{
			name: "simple formatted message",
			err:  E("fmted %s %t %d%d", "string", true, 13, 37),
			want: "fmted string true 1337",
		},
		{
			name: "error aware types are not use in the format",
			err:  E("fmted %s %t %d%d", "string", errors.Kind("test"), true, 13, 37),
			want: "test: fmted string true 1337",
		},
		{
			name: "all non recognized types are format args",
			err:  E(errors.Kind("test"), true, 13, 37, "fmted %t %d%d"),
			want: "test: fmted true 1337",
		},
		{
			name: "simple message with kind",
			err:  E(syntaxError, "failed to parse config"),
			want: fmt("%s: failed to parse config", syntaxError),
		},
		{
			name: "the kind of previous error is promoted if new one lacks it",
			err:  E("failed to parse config", E(syntaxError, "unexpected IDENT")),
			want: fmt("%s: failed to parse config: unexpected IDENT", syntaxError),
		},
		{
			name: "multiple different error kinds",
			err: E(
				tmSchemaError, "failed to parse config",
				E(tfSchemaError, "malformed terraform code"),
			),
			want: fmt(
				"%s: failed to parse config: %s: malformed terraform code",
				tmSchemaError, tfSchemaError,
			),
		},
		{
			name: "the file range gets promoted if current error lacks the file context",
			err: E("failed to parse config",
				E(tmSchemaError, hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				}, "unexpected attribute name"),
			),
			want: fmt("test.tm:1,5-10: %s: failed to parse config: unexpected attribute name",
				tmSchemaError),
		},
		{
			name: "nested errors",
			err:  E("1", E("2", E("3"))),
			want: "1: 2: 3",
		},
		{
			name: "nested errors with last one with range",
			err: E("1", E("2", E("3", hcl.Range{
				Filename: "test.tm",
				Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
				End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
			})),
			),
			want: "test.tm:1,5-10: 1: 2: 3",
		},
		{
			name: "single element diags sets range and description",
			err: E(hcl.Diagnostics{
				&hcl.Diagnostic{
					Detail:   "some error",
					Severity: hcl.DiagError,
					Subject: &hcl.Range{
						Filename: "test.tm",
						Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
				},
			}),
			want: "test.tm:1,5-10: some error",
		},
		{
			name: "single diag sets range and description",
			err: E(hcl.Diagnostic{
				Detail:   "some error",
				Severity: hcl.DiagError,
				Subject: &hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				},
			},
			),
			want: "test.tm:1,5-10: some error",
		},
		{
			name: "single diag pointer sets range and description",
			err: E(&hcl.Diagnostic{
				Detail:   "some error",
				Severity: hcl.DiagError,
				Subject: &hcl.Range{
					Filename: "test.tm",
					Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
					End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
				},
			},
			),
			want: "test.tm:1,5-10: some error",
		},
		{
			name: "nil diag pointer is ignored",
			err:  E("error", nildiagnostic),
			want: "error",
		},
		{
			name: "simple message with stack",
			err: E(syntaxError, "failed to parse config", stackmeta{
				name: "test",
				path: "/test",
				desc: "test desc",
			}),
			want: fmt("%s: failed to parse config: at stack \"/test\"", syntaxError),
		},
		{
			name: "underlying error list with single element",
			err:  E(syntaxError, errors.L(stderrors.New("err"))),
			want: fmt("%s: err", syntaxError),
		},
		{
			name: "underlying error list with multiple elements",
			err:  E(syntaxError, errors.L(stderrors.New("err1"), stderrors.New("err2"))),
			want: fmt("%s: err1 (and 1 elided errors)", syntaxError),
		},
		{
			name: "multiple underlying error builds a list",
			err:  E(syntaxError, stderrors.New("err1"), stderrors.New("err2")),
			want: fmt("%s: err1 (and 1 elided errors)", syntaxError),
		},
		{
			name: "hcl.Diagnostics builds a list of hcl.Diagnostic errors",
			err: E(syntaxError, hcl.Diagnostics{
				&hcl.Diagnostic{
					Detail:   "err 1",
					Severity: hcl.DiagError,
					Subject: &hcl.Range{
						Filename: "test.tm",
						Start:    hcl.Pos{Line: 1, Column: 3, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
				},
				&hcl.Diagnostic{
					Detail:   "err 2",
					Severity: hcl.DiagError,
					Subject: &hcl.Range{
						Filename: "test2.tm",
						Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
				},
			}),
			want: fmt("test.tm:1,3-10: %s: err 1 (and 1 elided errors)", syntaxError),
		},
		{
			name: "non-error hcl.Diagnostic is ignored",
			err: E(syntaxError, hcl.Diagnostics{
				&hcl.Diagnostic{
					Detail:   "err 1",
					Severity: hcl.DiagWarning,
					Subject: &hcl.Range{
						Filename: "test.tm",
						Start:    hcl.Pos{Line: 1, Column: 3, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
				},
				&hcl.Diagnostic{
					Detail:   "err 2",
					Severity: hcl.DiagError,
					Subject: &hcl.Range{
						Filename: "test2.tm",
						Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
						End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
					},
				},
			}),
			want: fmt("test2.tm:1,5-10: %s: err 2", syntaxError),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualStrings(t, tc.want, tc.err.Error())
		})

		// piggyback on Error formatting tests for Errors list testing

		t.Run("errors list with single error/"+tc.name, func(t *testing.T) {
			errs := errors.L(tc.err)

			assert.EqualStrings(t, tc.want, errs.Error())
			assert.EqualStrings(t, tc.want, errs.AsError().Error())
		})
		t.Run("errors list with multiple errors/"+tc.name, func(t *testing.T) {
			errs := errors.L(tc.err, E("will be elided"))
			errs.Append(E("will also be elided"))
			want := fmt("%s (and %d elided errors)",
				errs.Errors()[0].Error(), len(errs.Errors())-1)

			assert.EqualStrings(t, want, errs.Error())
			assert.EqualStrings(t, want, errs.AsError().Error())
		})
	}
}

func TestErrorIs(t *testing.T) {
	type testcase struct {
		name    string
		err     error
		target  error
		areSame bool
	}

	stack := stackmeta{
		name: "stack",
		desc: "desc",
		path: "/stack",
	}
	otherStack := stackmeta{
		name: "otherstack",
		desc: "other desc",
		path: "/otherstack",
	}
	filerange := hcl.Range{
		Filename: "test.tm",
		Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
		End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
	}
	otherFileRange := hcl.Range{
		Filename: "other.tm",
		Start:    hcl.Pos{Line: 6, Column: 3, Byte: 4},
		End:      hcl.Pos{Line: 7, Column: 1, Byte: 10},
	}

	for _, tc := range []testcase{
		{
			name:   "nil target is not equal",
			err:    E("any error"),
			target: nil,
		},
		{
			name:   "*error.Error with no underlying error is not comparable with stderrors",
			err:    E("any error"),
			target: stderrors.New("any error"),
		},
		{
			name:    "same description",
			err:     E("any error"),
			target:  E("any error"),
			areSame: true,
		},
		{
			name:    "same wrapped description",
			err:     E("msg", E("any error")),
			target:  E("any error"),
			areSame: true,
		},
		{
			name:    "same kind",
			err:     E(syntaxError, "error"),
			target:  E(syntaxError),
			areSame: true,
		},
		{
			name:    "same underlying kind",
			err:     E("error", E(syntaxError)),
			target:  E(syntaxError),
			areSame: true,
		},
		{
			name:    "same underlying kind (deep nested)",
			err:     E("error", E(tfSchemaError, E(tmSchemaError, E(syntaxError)))),
			target:  E(syntaxError),
			areSame: true,
		},
		{
			name:    "same stack",
			err:     E("error", stack),
			target:  E(stack),
			areSame: true,
		},
		{
			name:    "same underlying stack",
			err:     E("error", E(stack)),
			target:  E(stack),
			areSame: true,
		},
		{
			name:    "same underlying stack (deep nested)",
			err:     E("error", E(otherStack, E("msg", E(stack)))),
			target:  E(stack),
			areSame: true,
		},
		{
			name:    "same file range",
			err:     E("error", filerange),
			target:  E(filerange),
			areSame: true,
		},
		{
			name: "same file range built from hcl.Diagnostic",
			err: E("error", hcl.Diagnostic{
				Detail:   "some error",
				Severity: hcl.DiagError,
				Subject:  &filerange,
			}),
			target:  E(filerange),
			areSame: true,
		},
		{
			name:    "same underlying stack",
			err:     E("error", E(filerange)),
			target:  E(filerange),
			areSame: true,
		},
		{
			name:    "same underlying stack (deep nested)",
			err:     E("error", E(otherFileRange, E("msg", E(filerange)))),
			target:  E(filerange),
			areSame: true,
		},
		{
			name:    "underlying error is a stderror",
			err:     E("some error wrapping a stderror", os.ErrNotExist),
			target:  os.ErrNotExist,
			areSame: true,
		},
		{
			name:    "same file range",
			err:     E("error", filerange),
			target:  E("error", filerange),
			areSame: true,
		},
		{
			name:   "different file ranges",
			err:    E("error", filerange),
			target: E("error", otherFileRange),
		},
		{
			name:    "error match wrapped on stderr",
			err:     stdfmt.Errorf("stderr : %w", E(syntaxError)),
			target:  E(syntaxError),
			areSame: true,
		},
	} {

		assertErrorIsTarget := func(t *testing.T, err error) {
			t.Helper()

			res := errors.Is(err, tc.target)
			if res != tc.areSame {
				t.Fatalf("error[%v] == target[%v] = %t but want %t",
					err, tc.target, res, tc.areSame)
			}
		}

		t.Run(tc.name, func(t *testing.T) {
			assertErrorIsTarget(t, tc.err)
		})

		//piggyback on errors.Error tests

		t.Run("errors list with single err/"+tc.name, func(t *testing.T) {
			errs := errors.L(tc.err)
			assertErrorIsTarget(t, errs)
		})

		t.Run("errors list match is first/"+tc.name, func(t *testing.T) {
			errs := errors.L(tc.err, stderrors.New("not match"))
			assertErrorIsTarget(t, errs)
		})

		t.Run("errors list match is last/"+tc.name, func(t *testing.T) {
			errs := errors.L(stderrors.New("not match"), tc.err)
			assertErrorIsTarget(t, errs)
		})
	}
}

func TestErrorRangeRepresentation(t *testing.T) {
	t.Run("valid abspath", func(t *testing.T) {
		filerange := hcl.Range{
			Filename: "/test/test.tm",
			Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
			End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
		}
		err := E(filerange)
		assert.EqualStrings(t, "/test/test.tm:1,5-10", err.Error())
	})

	t.Run("valid relpath", func(t *testing.T) {
		filerange := hcl.Range{
			Filename: "name/test.tm.hcl",
			Start:    hcl.Pos{Line: 2, Column: 5, Byte: 3},
			End:      hcl.Pos{Line: 2, Column: 10, Byte: 13},
		}
		err := E(filerange)
		assert.EqualStrings(t, "name/test.tm.hcl:2,5-10", err.Error())
	})

	t.Run("valid abspath with any suffix", func(t *testing.T) {
		filerange := hcl.Range{
			Filename: "/test/test.tf",
			Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
			End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
		}
		err := E(filerange)
		assert.EqualStrings(t, "/test/test.tf:1,5-10", err.Error())
	})

	t.Run("invalid path containing zeroes", func(t *testing.T) {
		filerange := hcl.Range{
			Filename: string([]byte{'a', 0, 'b'}),
			Start:    hcl.Pos{Line: 2, Column: 5, Byte: 3},
			End:      hcl.Pos{Line: 2, Column: 10, Byte: 13},
		}
		err := E(filerange)
		assert.EqualStrings(t, "<generated-code>:2,5-10", err.Error())
	})
}

func TestDetailedRepresentation(t *testing.T) {
	stack := stackmeta{
		name: "stack",
		desc: "desc",
		path: "/stack",
	}
	filerange := hcl.Range{
		Filename: "test.tm",
		Start:    hcl.Pos{Line: 1, Column: 5, Byte: 3},
		End:      hcl.Pos{Line: 1, Column: 10, Byte: 13},
	}

	var e *errors.Error
	err := E("error", stack, filerange, errors.L(E("error")))
	errors.As(err, &e)

	if e.Error() == e.Detailed() {
		t.Error("detailed error should be different than default")
		t.Fatalf("instead both are: %s", e.Error())
	}
}

func fmt(format string, args ...interface{}) string {
	return stdfmt.Sprintf(format, args...)
}

type stackmeta struct {
	name string
	desc string
	path string
}

func (s stackmeta) Name() string { return s.name }
func (s stackmeta) Path() string { return s.path }
func (s stackmeta) Desc() string { return s.desc }
