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

package hcl_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/hcl"
)

func TestFormatHCL(t *testing.T) {
	type testcase struct {
		name  string
		input string
		want  string
	}

	tcases := []testcase{
		{
			name: "attributes alignment",
			input: `
a = 1
 b = "la"
	c = 666
  d = []
`,
			want: `
a = 1
b = "la"
c = 666
d = []
`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := hcl.Format(tcase.input)
			if diff := cmp.Diff(got, tcase.want); diff != "" {
				t.Fatalf("-(got) +(want):\n%s", diff)
			}
		})
	}
}
