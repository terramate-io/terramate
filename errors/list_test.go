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
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
)

func TestEmptyErrorListReturnsEmptyErrors(t *testing.T) {
	e := errors.L()
	errs := e.Errors()

	assert.EqualInts(t, 0, len(errs))
}

func TestErrorListReturnsAllErrors(t *testing.T) {
	e := errors.L()

	assert.EqualInts(t, 0, len(e.Errors()))

	e.Append(E("one"))
	e.Append(stdfmt.Errorf("wrapped: %w", E("two")))
	e.Append(stderrors.New("ignored"))
	e.Append(E("three"))

	errs := e.Errors()

	assert.EqualInts(t, 3, len(errs))
	assert.IsError(t, errs[0], E("one"))
	assert.IsError(t, errs[1], E("two"))
	assert.IsError(t, errs[2], E("three"))
}

func TestEmptyErrorListStringRepresentationIsEmpty(t *testing.T) {
	errs := errors.L()
	assert.EqualStrings(t, "", errs.Error())
	assert.EqualStrings(t, "", errs.Detailed())
}

func TestEmptyErrorListAsErrorIsNil(t *testing.T) {
	errs := errors.L()
	err := errs.AsError()
	if err != nil {
		t.Fatalf("got error %v but want nil", err)
	}
}

func TestErrorListIgnoresNilErrors(t *testing.T) {
	errs := errors.L(nil, nil)
	errs.Append(nil)
	err := errs.AsError()
	if err != nil {
		t.Fatalf("got error %v but want nil", err)
	}
}

func TestErrorListStringDetailedPresentation(t *testing.T) {
	errs := errors.L(E("one"))
	assert.EqualStrings(t, "error list:\n\t-one", errs.Detailed())

	errs.Append(E("two"))
	assert.EqualStrings(t, "error list:\n\t-one\n\t-two", errs.Detailed())
}
