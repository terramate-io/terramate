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

package dynrange_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl/dynexpr/dynrange"
)

func TestInjectionReplace(t *testing.T) {
	for _, tc := range []struct {
		before  string
		after   string
		expr    string
		replace string
		want    string
	}{
		{
			before: "",
			after:  "",
			expr:   "a",
			want:   "",
		},
		{
			before: "AAA",
			after:  "",
			expr:   "a",
			want:   "AAA",
		},
		{
			before: "",
			after:  "AAA",
			expr:   "a",
			want:   "AAA",
		},
		{
			before: "AAA",
			after:  "BBB",
			expr:   "a",
			want:   "AAABBB",
		},
		{
			before:  "AAA",
			after:   "BBB",
			expr:    "a",
			replace: "CCC",
			want:    "AAACCCBBB",
		},
	} {
		wrapped := dynrange.WrapExprBytes([]byte(tc.expr))
		if !dynrange.HasExprBytesWrapped(wrapped) {
			t.Fatal("has no injection")
		}
		got, _ := dynrange.UnwrapExprBytes(wrapped)
		assert.EqualStrings(t, tc.expr, string(got))
		intertwined := tc.before + wrapped + tc.after
		assert.EqualStrings(t, tc.want, dynrange.ReplaceInjectedExpr(intertwined, tc.replace))
	}
}
