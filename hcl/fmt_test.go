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
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"

	errtest "github.com/mineiros-io/terramate/test/errors"
)

// TODO(katcipis):
// - List of objects with lists inside
// - Comments inside lists
// - Blocks and subblocks with lists inside

func TestFormatHCL(t *testing.T) {
	type testcase struct {
		name     string
		input    string
		want     string
		wantErrs []error
	}

	tcases := []testcase{
		{
			name: "empty",
		},
		{
			name:  "only newlines are preserved",
			input: "\n\n\n",
			want:  "\n\n\n",
		},
		{
			name:  "only spaces are preserved",
			input: "  ",
			want:  "  ",
		},
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
d = [
]
`,
		},
		{
			name: "ternary with list comprehension",
			input: `
var = 1 >= global.num ? local.x : [   for x in    local.a : x  ]
`,
			want: `
var = 1 >= global.num ? local.x : [for x in local.a : x]
`,
		},
		{
			name: "assignment with list comprehension",
			input: `
var = [   for x in    local.a : x  ]
`,
			want: `
var = [for x in local.a : x]
`,
		},
		{
			name: "multi line list comprehension",
			input: `
var = [
for x in    local.a : x
]
`,
			want: `
var = [
  for x in local.a : x
]
`,
		},
		{
			name: "assignment with map comprehension",
			input: `
var = {  for s    in var.list : s =>     upper(s)    }
`,
			want: `
var = { for s in var.list : s => upper(s) }
`,
		},
		{
			name: "empty list",
			input: `
var = []
`,
			want: `
var = [
]
`,
		},
		{
			name: "empty lists all the way down",
			input: `
var = [[[[]]]]
`,
			want: `
var = [
  [
    [
      [
      ],
    ],
  ],
]
`,
		},
		{
			name: "list as operands",
			input: `
var = [ "item" ] + [ true ]
`,
			// The want here is non-ideal but it is what we get today
			// using hcl.Format. Fixing this would take even more work.
			want: `
var = [
  "item",
  ] + [
  true,
]
`,
		},
		{
			name: "list operated with other things",
			input: `
var = [ [ "item" ] + 1, [ "item" ] + true, [ "item" ] + {"test":true, "hi": "test"} ]
`,
			want: `
var = [
  [
    "item",
  ] + 1,
  [
    "item",
  ] + true,
  [
    "item",
  ] + { "test" : true, "hi" : "test" },
]
`,
		},
		{
			name: "nested list as operands",
			input: `
var = [[ "item" ] + [ true ]]
`,
			// The want here is non-ideal but it is what we get today
			// using hcl.Format. Fixing this would take even more work.
			want: `
var = [
  [
    "item",
    ] + [
    true,
  ],
]
`,
		},
		{
			name: "list indexing",
			input: `
var = [ "item" ][0]
`,
			want: `
var = [
  "item",
][0]
`,
		},
		{
			name: "nested list indexing",
			input: `
var = [["item"][0],["nesting"][666]]
`,
			want: `
var = [
  [
    "item",
  ][0],
  [
    "nesting",
  ][666],
]
`,
		},
		{
			name: "single item list",
			input: `
var = [ "item" ]
`,
			want: `
var = [
  "item",
]
`,
		},
		{
			name: "function call with list as parameter",
			input: `
var = func([1,2,3])
`,
			want: `
var = func([1, 2, 3])
`,
		},
		{
			name: "single item list with function call",
			input: `
var = [ func(local.a) ]
`,
			want: `
var = [
  func(local.a),
]
`,
		},
		{
			name: "single item list with nested function call",
			input: `
var = [ func(func(local.a)) ]
`,
			want: `
var = [
  func(func(local.a)),
]
`,
		},
		{
			name: "multiple items with function calls",
			input: `
var = [ func(local.a), f(local.a[1][2]) ]
`,
			want: `
var = [
  func(local.a),
  f(local.a[1][2]),
]
`,
		},
		{
			name: "single item list with index access",
			input: `
var = [ local.a[0] ]
`,
			want: `
var = [
  local.a[0],
]
`,
		},
		{
			name: "single item list with two dimension index access",
			input: `
var = [ local.a[0][1] ]
`,
			want: `
var = [
  local.a[0][1],
]
`,
		},
		{
			name: "multiple item list with index access",
			input: `
var = [ local.a[0], local.b[1]]
`,
			want: `
var = [
  local.a[0],
  local.b[1],
]
`,
		},
		{
			name: "nested list with index access",
			input: `
var = [[local.a[0]],[local.b["name"]]]
`,
			want: `
var = [
  [
    local.a[0],
  ],
  [
    local.b["name"],
  ],
]
`,
		},
		{
			name: "multiple item list",
			input: `
var = [ true, false, true ]
`,
			want: `
var = [
  true,
  false,
  true,
]
`,
		},
		{
			name: "adds comma on last element",
			input: `
var = [
  true,
  false,
  true
]
`,
			want: `
var = [
  true,
  false,
  true,
]
`,
		},
		{
			name: "list with lists and values intertwined",
			input: `
var = [
  true,
  [1],
  666,
  [6],
  "hi"
]
`,
			want: `
var = [
  true,
  [
    1,
  ],
  666,
  [
    6,
  ],
  "hi",
]
`,
		},
		{
			name: "multiple item list with objects",
			input: `
var = [ {name="test1"}, {name="test2"} ]
`,
			want: `
var = [
  { name = "test1" },
  { name = "test2" },
]
`,
		},
		{
			name: "multiple item list with objects and newlines/spaces",
			input: `
var = [ {name="test1"}     
,
{name="test2"}      

,


{name="test3"}]
`,
			want: `
var = [
  { name = "test1" },
  { name = "test2" },
  { name = "test3" },
]
`,
		},
		{
			name: "list with object with multiple keys",
			input: `
var = [{name="test1",x="hi"}]
`,
			want: `
var = [
  { name = "test1", x = "hi" },
]
`,
		},
		{
			name: "list with lists with object with multiple keys",
			input: `
var = [ [{name="test1",x="hi"}],[{name="test2",x="hi"}],[{name="test3",x="hi"}]
]
`,
			want: `
var = [
  [
    { name = "test1", x = "hi" },
  ],
  [
    { name = "test2", x = "hi" },
  ],
  [
    { name = "test3", x = "hi" },
  ],
]
`,
		},
		{
			name: "list of lists",
			input: `
var = [[1], [1,2,3], [["hi","nesting","is","fun"]]]
`,
			want: `
var = [
  [
    1,
  ],
  [
    1,
    2,
    3,
  ],
  [
    [
      "hi",
      "nesting",
      "is",
      "fun",
    ],
  ],
]
`,
		},
		{
			name: "list of lists with newlines",
			input: `
var = [
[1],

[1,

2,3],

[

["hi",

"nesting",

"is",

"fun",


],

],

]
`,
			want: `
var = [
  [
    1,
  ],
  [
    1,
    2,
    3,
  ],
  [
    [
      "hi",
      "nesting",
      "is",
      "fun",
    ],
  ],
]
`,
		},
		{
			name: "multiple item list with string templates with commas",
			input: `
var = [ {name="${hi},${comma}"}, {name="${hi} [${hello}]"} ]
`,
			want: `
var = [
  { name = "${hi},${comma}" },
  { name = "${hi} [${hello}]" },
]
`,
		},
		{
			name: "fails on syntax errors",
			input: `
				string = hi"
				bool   = rue
				list   = [
				obj    = {
			`,
			wantErrs: []error{
				errors.E(hcl.ErrHCLSyntax),
				errors.E(mkrange(start(2, 17, 17), end(3, 1, 18))),
				errors.E(mkrange(start(3, 17, 34), end(4, 1, 35))),
				errors.E(mkrange(start(4, 15, 49), end(5, 1, 50))),
				errors.E(mkrange(start(5, 15, 64), end(6, 1, 65))),
				errors.E(mkrange(start(2, 16, 16), end(2, 17, 17))),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			const filename = "test-input.hcl"
			got, err := hcl.Format(tcase.input, filename)

			addFilenameToErrorsFileRanges(tcase.wantErrs, filename)
			errtest.AssertErrorList(t, err, tcase.wantErrs)
			assert.EqualStrings(t, tcase.want, got)

			if err != nil {
				return
			}

			got2, err := hcl.Format(got, "formatted.hcl")
			assert.NoError(t, err)
			assert.EqualStrings(t, got, got2, "reformatting should produce identical results")
		})

		if tcase.input == tcase.want {
			// We dont test FormatTree for no formatting changes here.
			// Only scenarios where changes will be detected.
			continue
		}

		// piggyback on the overall formatting scenarios to check
		// for hcl.FormatTree behavior.
		t.Run("Tree/"+tcase.name, func(t *testing.T) {
			const (
				filename   = "file.tm"
				subdirName = "subdir"
			)

			rootdir := t.TempDir()
			test.Mkdir(t, rootdir, subdirName)
			subdir := filepath.Join(rootdir, subdirName)

			test.WriteFile(t, rootdir, filename, tcase.input)
			test.WriteFile(t, subdir, filename, tcase.input)

			got, err := hcl.FormatTree(rootdir)

			// Since we have identical files we expect the same
			// set of errors for each filepath to be present.
			wantFilepath := filepath.Join(rootdir, filename)
			wantSubdirFilepath := filepath.Join(subdir, filename)
			wantErrs := []error{}

			for _, path := range []string{wantFilepath, wantSubdirFilepath} {
				for _, wantErr := range tcase.wantErrs {
					if e, ok := wantErr.(*errors.Error); ok {
						err := *e
						err.FileRange.Filename = path
						wantErrs = append(wantErrs, &err)
						continue
					}

					wantErrs = append(wantErrs, wantErr)
				}

			}
			errtest.AssertErrorList(t, err, wantErrs)
			if err != nil {
				return
			}
			assert.EqualInts(t, 2, len(got), "want 2 formatted files, got: %v", got)

			for _, res := range got {
				assert.EqualStrings(t, tcase.want, res.Formatted())
				assertFileContains(t, res.Path(), tcase.input)
			}

			assert.EqualStrings(t, wantFilepath, got[0].Path())
			assert.EqualStrings(t, wantSubdirFilepath, got[1].Path())

			t.Run("saving format results", func(t *testing.T) {
				for _, res := range got {
					assert.NoError(t, res.Save())
					assertFileContains(t, res.Path(), res.Formatted())
				}

				got, err := hcl.FormatTree(rootdir)
				assert.NoError(t, err)

				if len(got) > 0 {
					t.Fatalf("after formatting want 0 fmt results, got: %v", got)
				}
			})
		})
	}
}

func TestFormatTreeReturnsEmptyResultsForEmptyDir(t *testing.T) {
	tmpdir := t.TempDir()
	got, err := hcl.FormatTree(tmpdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

func TestFormatTreeFailsOnNonExistentDir(t *testing.T) {
	tmpdir := t.TempDir()
	_, err := hcl.FormatTree(filepath.Join(tmpdir, "non-existent"))
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonAccessibleSubdir(t *testing.T) {
	const subdir = "subdir"
	tmpdir := t.TempDir()
	test.Mkdir(t, tmpdir, subdir)

	assert.NoError(t, os.Chmod(filepath.Join(tmpdir, subdir), 0))

	_, err := hcl.FormatTree(tmpdir)
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonAccessibleFile(t *testing.T) {
	const filename = "filename.tm"

	tmpdir := t.TempDir()
	test.WriteFile(t, tmpdir, filename, `globals{
	a = 2
		b = 3
	}`)

	assert.NoError(t, os.Chmod(filepath.Join(tmpdir, filename), 0))

	_, err := hcl.FormatTree(tmpdir)
	assert.Error(t, err)
}

func TestFormatTreeIgnoresNonTerramateFiles(t *testing.T) {
	const (
		subdirName      = ".dotdir"
		unformattedCode = `
a = 1
 b = "la"
	c = 666
  d = []
`
	)

	tmpdir := t.TempDir()
	test.WriteFile(t, tmpdir, ".file.tm", unformattedCode)
	test.WriteFile(t, tmpdir, "file.tf", unformattedCode)
	test.WriteFile(t, tmpdir, "file.hcl", unformattedCode)

	test.Mkdir(t, tmpdir, subdirName)
	subdir := filepath.Join(tmpdir, subdirName)
	test.WriteFile(t, subdir, ".file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm.hcl", unformattedCode)

	got, err := hcl.FormatTree(tmpdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

func assertFileContains(t *testing.T, filepath, got string) {
	t.Helper()

	data, err := os.ReadFile(filepath)
	assert.NoError(t, err, "reading file")

	want := string(data)
	assert.EqualStrings(t, want, got, "file %q contents don't match", filepath)
}
