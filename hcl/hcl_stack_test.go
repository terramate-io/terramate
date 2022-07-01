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
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
)

func TestHCLParserStackID(t *testing.T) {
	validIDs := []string{
		"_",
		"-",
		"_id_",
		"-id-",
		"_id_again_",
		"-id-again-",
		"-id_mixed-",
		"-id_numbers-0123456789-",
		"maxsize_id_Test_should_Be_64_bytes_aNd_now_running_out_of_ID-aaa",
	}
	invalidIDs := []string{
		"",
		"*not+valid$",
		"cachaça",
		"maxsize_id_test_should_be_64_bytes_and_now_running_out_of_id-aaac",
	}

	testcases := []testcase{}

	stackBlock := func(id string) string {
		return fmt.Sprintf(`
			stack {
				id = %q
			}
			`, id)
	}
	newStackID := func(id string) hcl.StackID {
		t.Helper()
		v, err := hcl.NewStackID(id)
		assert.NoError(t, err)
		return v
	}

	for _, validID := range validIDs {
		testcases = append(testcases, testcase{
			name: fmt.Sprintf("stack ID %s is valid", validID),
			input: []cfgfile{
				{
					filename: "stack.tm",
					body:     stackBlock(validID),
				},
			},
			want: want{
				config: hcl.Config{
					Stack: &hcl.Stack{
						ID: newStackID(validID),
					},
				},
			},
		})
	}

	for _, invalidID := range invalidIDs {
		testcases = append(testcases, testcase{
			name: fmt.Sprintf("stack ID %s is invalid", invalidID),
			input: []cfgfile{
				{
					filename: "stack.tm",
					body:     stackBlock(invalidID),
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema),
				},
			},
		})
	}

	for _, tc := range testcases {
		testParser(t, tc)
	}
}

func TestHCLParserStack(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "empty stack block",
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
			name: "empty name",
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
						mkrange("stack.tm", start(6, 8, 77), end(6, 12, 81)),
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
						mkrange("stack.tm", start(3, 8, 22), end(3, 10, 24)),
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
						mkrange("stack.tm", start(6, 18, 87), end(6, 22, 91))),
					errors.E(hcl.ErrTerramateSchema,
						mkrange("stack.tm", start(6, 8, 77), end(6, 12, 81))),
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
			name: "after: empty set works",
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
			name: "'after' single entry",
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
			name: "multiple 'before' fields - fails",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						stack {
							before = []
							before = []
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
			name: "'before' single entry",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something"},
					},
				},
			},
		},
		{
			name: "'before' multiple entries",
			input: []cfgfile{
				{
					filename: "stack.tm",
					body: `
						terramate {
							required_version = ""
						}

						stack {
							before = ["something", "something-else", "test"]
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{},
					Stack: &hcl.Stack{
						Before: []string{"something", "something-else", "test"},
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
			name: "'before' and 'after'",
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
