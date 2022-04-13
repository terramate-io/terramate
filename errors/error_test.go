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

var (
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

func TestErrorString(t *testing.T) {
	type testcase struct {
		name string
		err  error
		want string
	}

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
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualStrings(t, tc.want, tc.err.Error())
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
			name:    "underlying error is of the provided stderror",
			err:     E("some error wrapping a stderror", os.ErrNotExist),
			target:  os.ErrNotExist,
			areSame: true,
		},
		{
			name:    "same file range",
			err:     E("error", hcl.Range{Filename: "test.hcl"}),
			target:  E("error", hcl.Range{Filename: "test.hcl"}),
			areSame: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := errors.Is(tc.err, tc.target)
			if res != tc.areSame {
				t.Fatalf("error[%v] == target[%v] = %t but want %t",
					tc.err, tc.target, res, tc.areSame)
			}
		})
	}
}

func fmt(format string, args ...interface{}) string {
	return stdfmt.Sprintf(format, args...)
}
