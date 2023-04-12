// Copyright 2023 Mineiros GmbH
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

package ast_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/ast"
)

func TestLabels(t *testing.T) {
	type testcase struct {
		in         []string
		serialized string
	}

	for _, tc := range []testcase{
		{
			in:         []string{},
			serialized: "",
		},
		{
			in:         []string{"a"},
			serialized: "1:a",
		},
		{
			in:         []string{"a"},
			serialized: "1:a",
		},
		{
			in:         []string{"a", "abc"},
			serialized: "1:a3:abc",
		},
		{
			in:         []string{":", "1:bc"},
			serialized: "1::4:1:bc",
		},
	} {
		s := ast.NewSerializedLabels(tc.in)
		assert.EqualStrings(t, tc.serialized, string(s))
		out := s.Unserialize()
		assert.EqualInts(t, len(tc.in), len(out))
		for i, got := range out {
			assert.EqualStrings(t, tc.in[i], got)
		}
	}
}
