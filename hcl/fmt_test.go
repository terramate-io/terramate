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
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
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
			assert.EqualStrings(t, tcase.want, got)
		})

		const filename = "file.tm"

		tmpdir := t.TempDir()
		test.WriteFile(t, tmpdir, filename, tcase.input)

		t.Run("File/"+tcase.name, func(t *testing.T) {
			got, err := hcl.FormatFile(filepath.Join(tmpdir, filename))
			assert.NoError(t, err)
			assert.EqualStrings(t, tcase.want, got)
		})
	}
}

func TestFormatFileDoesntExist(t *testing.T) {
	tmpdir := t.TempDir()
	_, err := hcl.FormatFile(filepath.Join(tmpdir, "dontexist.tm"))
	assert.Error(t, err)
}
