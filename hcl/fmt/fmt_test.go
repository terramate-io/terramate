// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/terramate-io/terramate/test"

	. "github.com/terramate-io/terramate/test/hclutils"

	errtest "github.com/terramate-io/terramate/test/errors"
)

func TestFormatMultiline(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name     string
		input    string
		want     string
		wantErrs []error
	}

	const filename = "test-input.hcl"

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
			name: "function call with list comprehension",
			input: `
var = f([   for x in    local.a : x  ])
`,
			want: `
var = f([for x in local.a : x])
`,
		},
		{
			name: "object with list comprehension",
			input: `
var = { a = [   for x in    local.a : x  ] }
`,
			want: `
var = { a = [for x in local.a : x] }
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
			name: "function call with multi line list comprehension",
			input: `
var = f([
for x in    local.a : x
])
`,
			want: `
var = f([
  for x in local.a : x
])
`,
		},
		{
			name: "object with multi line list comprehension",
			input: `
var = { a = [
for x in    local.a : x
] }
`,
			want: `
var = { a = [
  for x in local.a : x
] }
`,
		},
		{
			name: "list with object with multi line list key",
			input: `
var = [ { [
] = 0 },
]
`,
			want: `
var = [
  { [
  ] = 0 },
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
			name: "function call with map comprehension",
			input: `
var = f({  for s    in var.list : s =>     upper(s)    })
`,
			want: `
var = f({ for s in var.list : s => upper(s) })
`,
		},
		{
			name: "object with map comprehension",
			input: `
var = { a = {  for s    in var.list : s =>     upper(s)    } }
`,
			want: `
var = { a = { for s in var.list : s => upper(s) } }
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
			name: "funcall with empty list",
			input: `
var = f([])
`,
			want: `
var = f([
])
`,
		},
		{
			name: "object with empty list",
			input: `
var = { a = [] }
`,
			want: `
var = { a = [
  ]
}
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
			name: "list with single heredoc",
			input: `
var = [
<<EOT
hello
world
EOT
]
`,
			want: `
var = [
  <<EOT
hello
world
EOT
  ,
]
`,
		},
		{
			name: "function call with single heredoc",
			input: `
var = f([
<<EOT
hello
world
EOT
])
`,
			want: `
var = f([
  <<EOT
hello
world
EOT
  ,
])
`,
		},
		{
			name: "object with single heredoc",
			input: `
var = { a = [
<<EOT
hello
world
EOT
]}
`,
			want: `
var = { a = [
  <<EOT
hello
world
EOT
  ,
  ]
}
`,
		},
		{
			name: "heredoc with commas and []",
			input: `
var = [
<<EOT
hello,
world,
seems like a list [1,2,3]
EOT
]
`,
			want: `
var = [
  <<EOT
hello,
world,
seems like a list [1,2,3]
EOT
  ,
]
`,
		},
		{
			name: "list with multiple heredocs",
			input: `
var = [
<<EOT
hello
world
EOT
,
<<-EOT
hello
world2
EOT
]
`,
			want: `
var = [
  <<EOT
hello
world
EOT
  ,
  <<-EOT
hello
world2
EOT
  ,
]
`,
		},
		{
			name: "list with heredocs mixed with other types",
			input: `
var = [
<<-EOT
hello
world
EOT
, 666, "test",
<<-EOT
hello
world2
EOT
]
`,
			want: `
var = [
  <<-EOT
hello
world
EOT
  ,
  666,
  "test",
  <<-EOT
hello
world2
EOT
  ,
]
`,
		},
		{
			name: "list with comments in the end",
			input: `
var = [] // hi
`,
			want: `
var = [
] // hi
`,
		},
		{
			name: "list with comments and newlines after end inside objects",
			input: `
var = {
  a = []

  // c2

  /* c3 */

  b = 666 // c4

  // c5
}
`,
			want: `
var = {
  a = [
  ] // c2
  /* c3 */
  b = 666 // c4

  // c5
}
`,
		},
		{
			name: "list with list comprehension with multiline comments",
			input: `
var = [ [
// c1
/* c2 */
# c3
for x in local.a : x
// c4
/* c5 */
# c6
] ]
`,
			want: `
var = [
  [
    // c1
    /* c2 */
    # c3
    for x in local.a : x
    // c4
    /* c5 */
    # c6
  ],
]
`,
		},
		{
			name: "function call with list comprehension with multiline comments",
			input: `
var = f([ f([
// c1
/* c2 */
# c3
for x in local.a : x
// c4
/* c5 */
# c6
]) ])
`,
			want: `
var = f([
  f([
    // c1
    /* c2 */
    # c3
    for x in local.a : x
    // c4
    /* c5 */
    # c6
  ]),
])
`,
		},
		{
			name: "list with list comprehension as elements",
			input: `
var = [ [for x in local.a : x], [for x in local.a : x] ]
`,
			want: `
var = [
  [for x in local.a : x],
  [for x in local.a : x],
]
`,
		},
		{
			name: "list with string templates inside",
			input: `
var = [ "${hi}-]," , "${something}[,"]
`,
			want: `
var = [
  "${hi}-],",
  "${something}[,",
]
`,
		},
		{
			name: "list with multiline string templates inside",
			input: `
var = [
  "${[
    "${
      {
         a = [
           "more list",
         ]
      }
    }"
]}",
]
`,
			want: `
var = [
  "${[
    "${
      {
        a = [
          "more list",
        ]
      }
    }"
  ]}",
]
`,
		},
		{
			name: "list as operands",
			input: `
var = [ "item" ] + [ true ]
`,
			want: `
var = [
  "item",
  ] + [
  true,
]
`,
		},
		{
			name: "function with list as operands",
			input: `
var = f([ "item" ] + [ true ])
`,
			want: `
var = f([
  "item",
  ] + [
  true,
])
`,
		},
		{
			name: "nested empty lists as operands with newlines",
			input: `
var = [[]%
[]]
`,
			// The extra indentation when using operators is introduced
			// by hashicorp's hcl.Format function.
			want: `
var = [
  [
    ] % [
  ],
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
			// using hcl.Format. Fixing this would take more work.
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
			name: "list indexing with object mixed",
			input: `
var = [ "item" ][0].name.hi[1]
`,
			want: `
var = [
  "item",
][0].name.hi[1]
`,
		},
		{
			name: "list indexing using function calls and objects as indexes",
			input: `
var = [ "item" ][func(666)].name.hi[{a=666}]
`,
			want: `
var = [
  "item",
][func(666)].name.hi[{ a = 666 }]
`,
		},
		{
			name: "list indexing using function calls and objects with lists as indexes",
			input: `
var = [ "item" ][func([666])].name.hi[{a=[666]}]
`,
			want: `
var = [
  "item",
][func([666])].name.hi[{ a = [666] }]
`,
		},
		{
			name: "function call with indexing with object mixed",
			input: `
var = f([ "item" ][0].name.hi[1])
`,
			want: `
var = f([
  "item",
][0].name.hi[1])
`,
		},
		{
			name: "object with indexing with object mixed",
			input: `
var = { a = [ "item" ][0].name.hi[1] }
`,
			want: `
var = { a = [
  "item",
][0].name.hi[1] }
`,
		},
		{
			name: "nested list indexing with object mixed",
			input: `
var = [[ "item" ][0].name.hi[1], [ "item" ][0].name.hi[1]]
`,
			want: `
var = [
  [
    "item",
  ][0].name.hi[1],
  [
    "item",
  ][0].name.hi[1],
]
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
			name: "object with single item list",
			input: `
var = { a = [ "item" ] }
`,
			want: `
var = { a = [
  "item",
  ]
}
`,
		},
		{
			name: "object multiple attributes",
			input: `
var = {
  a = [ "item" ]
  b = [ "item" ]
  c = [ "item" ]
}
`,
			want: `
var = {
  a = [
    "item",
  ]
  b = [
    "item",
  ]
  c = [
    "item",
  ]
}
`,
		},
		{
			name: "object multiple attributes with different key types",
			input: `
var = {
  {}         = [ "item" ]
  666        = [ "item" ]
  "hclisfun" = [ "item" ]
  true       = [ "item" ]
  [1]        = [ "item" ]
  [2]        = [ "item" ]
}
`,
			want: `
var = {
  {} = [
    "item",
  ]
  666 = [
    "item",
  ]
  "hclisfun" = [
    "item",
  ]
  true = [
    "item",
  ]
  [1] = [
    "item",
  ]
  [2] = [
    "item",
  ]
}
`,
		},
		{
			name: "object multiple attributes with commas",
			input: `
var = {
  a = [ "item" ],
  b = [],
  c = [6,6,6],
}
`,
			want: `
var = {
  a = [
    "item",
  ],
  b = [
  ],
  c = [
    6,
    6,
    6,
  ],
}
`,
		},
		{
			name: "nested object with multiple item list",
			input: `
var = {
  nested = {
    a = [ "item" ],
    b = [{
      a=[1,2,3]
    }],
    c = [6,6,6],
  }
}
`,
			want: `
var = {
  nested = {
    a = [
      "item",
    ],
    b = [
      {
        a = [
          1,
          2,
          3,
        ]
      },
    ],
    c = [
      6,
      6,
      6,
    ],
  }
}
`,
		},
		{
			name: "lists inside blocks",
			input: `
block1 {
  var = [ "item" ]
}
block2 {
  block3 {
    var = [ "item" ]
    a = f([ "item" ])
    b = { a = ["hi"] }
  }
}
`,
			want: `
block1 {
  var = [
    "item",
  ]
}
block2 {
  block3 {
    var = [
      "item",
    ]
    a = f([
      "item",
    ])
    b = { a = [
      "hi",
      ]
    }
  }
}
`,
		},
		{
			name: "function call with list as parameter",
			input: `
var = func([1,2,3])
`,
			want: `
var = func([
  1,
  2,
  3,
])
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
			name: "list with function that has lists",
			input: `
var = [ func([1,2,3]), func([func([]), func([1,2])]) ]
`,
			want: `
var = [
  func([
    1,
    2,
    3,
  ]),
  func([
    func([
    ]),
    func([
      1,
      2,
    ]),
  ]),
]
`,
		},
		{
			name: "single item list with function call and multiple params",
			input: `
var = [ func(local.a, local.b, 666, "hi") ]
`,
			want: `
var = [
  func(local.a, local.b, 666, "hi"),
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
			name: "nested list with newline and comment before index",
			input: `
var = [[] # c1
[*]]
`,
			want: `
var = [
  [
  ] # c1
  [*],
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
			name: "function call with single item list with two dimension index access",
			input: `
var = func([ local.a[0][1] ])
`,
			want: `
var = func([
  local.a[0][1],
])
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
				errors.E(Mkrange(filename, Start(2, 17, 17), End(3, 1, 18))),
				errors.E(Mkrange(filename, Start(3, 17, 34), End(4, 1, 35))),
				errors.E(Mkrange(filename, Start(4, 15, 49), End(5, 1, 50))),
				errors.E(Mkrange(filename, Start(5, 15, 64), End(6, 1, 65))),
				errors.E(Mkrange(filename, Start(2, 16, 16), End(2, 17, 17))),
			},
		},
	}

	for _, tcase := range tcases {
		tcase := tcase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			tempdir := test.TempDir(t)

			got, err := fmt.FormatMultiline(tcase.input, filepath.Join(tempdir, filename))

			FixupFiledirOnErrorsFileRanges(tempdir, tcase.wantErrs)
			errtest.AssertErrorList(t, err, tcase.wantErrs)

			if diff := cmp.Diff(got, tcase.want); diff != "" {
				t.Errorf("got:\n%s", got)
				t.Errorf("want:\n%s", tcase.want)
				t.Error("diff:")
				t.Fatal(diff)
			}

			if err != nil {
				return
			}

			got2, err := fmt.FormatMultiline(got, "formatted.hcl")
			assert.NoError(t, err)
			assert.EqualStrings(t, got, got2, "reformatting should produce identical results")
		})
	}
}

func TestFormatHCL(t *testing.T) {
	type testcase struct {
		name     string
		input    string
		want     string
		wantErrs []error
	}

	const filename = "test-input.hcl"

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
d = []
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
				errors.E(fmt.ErrHCLSyntax),
				errors.E(Mkrange(filename, Start(2, 17, 17), End(3, 1, 18))),
				errors.E(Mkrange(filename, Start(3, 17, 34), End(4, 1, 35))),
				errors.E(Mkrange(filename, Start(4, 15, 49), End(5, 1, 50))),
				errors.E(Mkrange(filename, Start(5, 15, 64), End(6, 1, 65))),
				errors.E(Mkrange(filename, Start(2, 16, 16), End(2, 17, 17))),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got, err := fmt.Format(tcase.input, filename)
			errtest.AssertErrorList(t, err, tcase.wantErrs)
			assert.EqualStrings(t, tcase.want, got)

			if err != nil {
				return
			}

			got2, err := fmt.Format(got, "formatted.hcl")
			assert.NoError(t, err)
			assert.EqualStrings(t, got, got2, "reformatting should produce identical results")
		})

		if tcase.input == tcase.want {
			// We dont test FormatTree for no formatting changes here.
			// Only scenarios where changes will be detected.
			continue
		}

		checkResults := func(t *testing.T, res []fmt.FormatResult, wantFiles []string, tcase testcase, gotErr error) {
			wantErrs := []error{}

			for _, path := range wantFiles {
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
			errtest.AssertErrorList(t, gotErr, wantErrs)
			if gotErr != nil {
				return
			}
			assert.EqualInts(t, 2, len(res), "want %d formatted files, got: %v", len(wantFiles), res)

			for _, res := range res {
				assert.EqualStrings(t, tcase.want, res.Formatted())
				assertFileContains(t, res.Path(), tcase.input)
			}

			for i, wantFile := range wantFiles {
				assert.EqualStrings(t, wantFile, res[i].Path())
			}
		}

		saveFiles := func(t *testing.T, rootdir string, res []fmt.FormatResult) {
			for _, r := range res {
				assert.NoError(t, r.Save())
				assertFileContains(t, r.Path(), r.Formatted())
			}

			got, err := fmt.FormatTree(rootdir)
			assert.NoError(t, err)

			if len(got) > 0 {
				t.Fatalf("after formatting want 0 fmt results, got: %v", got)
			}
		}

		sandbox := func(t *testing.T) (string, []string) {
			const (
				filename   = "file.tm"
				subdirName = "subdir"
			)

			rootdir := test.TempDir(t)
			test.Mkdir(t, rootdir, subdirName)
			subdir := filepath.Join(rootdir, subdirName)

			wantFilepath := test.WriteFile(t, rootdir, filename, tcase.input)
			wantSubdirFilepath := test.WriteFile(t, subdir, filename, tcase.input)
			return rootdir, []string{wantFilepath, wantSubdirFilepath}
		}

		// piggyback on the overall formatting scenarios to check
		// for hcl.FormatTree behavior.
		t.Run("Tree/"+tcase.name, func(t *testing.T) {
			rootdir, files := sandbox(t)
			got, err := fmt.FormatTree(rootdir)
			checkResults(t, got, files, tcase, err)
			if err == nil {
				saveFiles(t, rootdir, got)
			}
		})

		// piggyback on the overall formatting scenarios to check
		// for hcl.FormatFiles behavior.
		t.Run("Files/"+tcase.name, func(t *testing.T) {
			rootdir, files := sandbox(t)
			got, err := fmt.FormatFiles(rootdir, files)
			checkResults(t, got, files, tcase, err)
			if err == nil {
				saveFiles(t, rootdir, got)
			}
		})
	}
}

func TestFormatTreeReturnsEmptyResultsForEmptyDir(t *testing.T) {
	tmpdir := test.TempDir(t)
	got, err := fmt.FormatTree(tmpdir)
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(got), "want no results, got: %v", got)
}

func TestFormatTreeFailsOnNonAccessibleSubdir(t *testing.T) {
	const subdir = "subdir"
	tmpdir := test.TempDir(t)
	test.Mkdir(t, tmpdir, subdir)

	test.AssertChmod(t, filepath.Join(tmpdir, subdir), 0)
	defer test.AssertChmod(t, filepath.Join(tmpdir, subdir), 0755)

	_, err := fmt.FormatTree(tmpdir)
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonAccessibleFile(t *testing.T) {
	const filename = "filename.tm"

	tmpdir := test.TempDir(t)
	test.WriteFile(t, tmpdir, filename, `globals{
	a = 2
		b = 3
	}`)

	test.AssertChmod(t, filepath.Join(tmpdir, filename), 0)
	defer test.AssertChmod(t, filepath.Join(tmpdir, filename), 0755)

	_, err := fmt.FormatTree(tmpdir)
	assert.Error(t, err)
}

func TestFormatTreeFailsOnNonExistentDir(t *testing.T) {
	tmpdir := test.TempDir(t)
	_, err := fmt.FormatTree(filepath.Join(tmpdir, "non-existent"))
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

	tmpdir := test.TempDir(t)
	test.WriteFile(t, tmpdir, ".file.tm", unformattedCode)
	test.WriteFile(t, tmpdir, "file.tf", unformattedCode)
	test.WriteFile(t, tmpdir, "file.hcl", unformattedCode)

	test.Mkdir(t, tmpdir, subdirName)
	subdir := filepath.Join(tmpdir, subdirName)
	test.WriteFile(t, subdir, ".file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm", unformattedCode)
	test.WriteFile(t, subdir, "file.tm.hcl", unformattedCode)

	got, err := fmt.FormatTree(tmpdir)
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
