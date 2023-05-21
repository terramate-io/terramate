package hcl_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	. "github.com/terramate-io/terramate/test/hclutils"
)

func TestHCLParserStack(t *testing.T) {
	for _, tc := range []testcase{
		{
			name:      "empty stack block with terramate block not in root works in nonStrict mode",
			nonStrict: true,
			parsedir:  "stack",
			input: []cfgfile{
				{
					filename: "stack/stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "empty stack block with terramate block in root works in strict mode",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "stack with unrecognized blocks",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack{
							block1 {}
							block2 {}
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "tm_vendor is not available on stack block",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack{
						  name = tm_vendor("github.com/terramate-io/terramate?ref=v2")
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(eval.ErrEval),
				},
			},
		},
		{
			name: "tm_hcl_expression is not available on stack block",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack{
						  name = tm_hcl_expression("ref")
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(eval.ErrEval),
				},
			},
		},
		{
			name: "multiple stack blocks",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {}
						stack{}
						stack{}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name:      "empty name",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = ""
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name: "stack attributes supports tm_ funcalls",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							name = tm_upper("stack-name")
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Name: "STACK-NAME",
					},
				},
			},
		},
		{
			name: "name is not a string - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = 1
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("stack.tm", Start(6, 15, 85), End(6, 16, 85)),
					),
				},
			},
		},
		{
			name: "name is not a string after evaluating function - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							name = tm_concat(["a", ""], ["b", "c"])
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("stack.tm", Start(3, 15, 29), End(3, 47, 61)),
					),
				},
			},
		},
		{
			name: "id is not a string - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							id = 1
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("stack.tm", Start(3, 13, 27), End(3, 14, 28)),
					),
				},
			},
		},
		{
			name: "name has interpolation - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							name = "${test}"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("stack.tm", Start(6, 18, 87), End(6, 22, 91))),
				},
			},
		},
		{
			name: "unrecognized attribute name - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							bleh = "a"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "schema not checked for files with syntax errors",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {
							wants =
							unrecognized = "test"
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
		{
			name:      "after: empty set works",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}
						stack {}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack:     &hcl.Stack{},
				},
			},
		},
		{
			name:      "'after' single entry",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						After: []string{"test"},
					},
				},
			},
		},
		{
			name: "'after' invalid element entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = [1]
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "'after' referencing terramate.stack.list - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = terramate.stacks.list
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "'after' invalid type",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = {}
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "'after' null value",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = null
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						After: []string{},
					},
				},
			},
		},
		{
			name: "'after' duplicated entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = ["test", "test"]
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		},
		{
			name: "multiple 'after' fields - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							after = ["test"]
							after = []
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrHCLSyntax),
				},
			},
		},
		{
			name:      "'after' single entry",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["something"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						After: []string{"something"},
					},
				},
			},
		},
		{
			name:      "'after' multiple entries",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							after = ["something", "something-else", "test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						After: []string{"something", "something-else", "test"},
					},
				},
			},
		},
		{
			name: "stack with valid description",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							description = "some cool description"
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "some cool description",
					},
				},
			},
		},
		{
			name: "stack with multiline description",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
					stack {
						description =  <<-EOD
	line1
	line2
	EOD
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						Description: "line1\nline2",
					},
				},
			},
		},
		{
			name:      "'before' and 'after'",
			nonStrict: true,
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
							after = ["else"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
						After:  []string{"else"},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
