// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl_test

import (
	"testing"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	. "github.com/terramate-io/terramate/test/hclutils"
)

func TestHCLImport(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "import with label - fails",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `import "something" {
						source = "bleh"
				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(1, 8, 7), End(1, 19, 18))),
				},
			},
		},
		{
			name: "import missing source attribute - fails",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `import {

				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(1, 8, 7), End(1, 8, 7))),
				},
			},
		},
		{
			name:     "import with non-existent file - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/non-existent-file"
				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("stack/cfg.tm", Start(2, 16, 24), End(2, 42, 50))),
				},
			},
		},
		{
			name: "import same file - fails",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `import {
						source = "cfg.tm"
				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("cfg.tm", Start(2, 16, 24), End(2, 24, 32))),
				},
			},
		},
		{
			name: "import same directory - fails",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `import {
						source = "other.tm"
				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("cfg.tm", Start(2, 16, 24), End(2, 26, 34))),
				},
			},
		},
		{
			name:     "import cycle - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/cfg.tm"
				}`,
				},
				{
					filename: "other/cfg.tm",
					body: `import {
						source = "/stack/cfg.tm"
				}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("other/cfg.tm", Start(2, 16, 24), End(2, 31, 39))),
				},
			},
		},
		{
			name:     "import same tree - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/cfg.tm"
				}`,
				},
				{
					filename: "cfg.tm",
					body: `terramate {
					}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("stack/cfg.tm", Start(2, 16, 24), End(2, 25, 33))),
				},
			},
		},
		{
			name:     "import same file multiple times - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `
						import {
							source = "/other/cfg.tm"
						}
				
						import {
							source = "/other/cfg.tm"
						}
					`,
				},
				{
					filename: "other/cfg.tm",
					body:     `globals {}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("stack/cfg.tm", Start(7, 17, 92), End(7, 32, 107))),
				},
			},
		},
		{
			name:     "imported file imports same file multiple times - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/cfg.tm"
					}`,
				},
				{
					filename: "other/cfg.tm",
					body: `import {
						source = "/other2/cfg.tm"	
					}
					
					import {
						source = "/other2/cfg.tm"	
					}`,
				},
				{
					filename: "other2/cfg.tm",
					body:     `globals {}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("other/cfg.tm", Start(6, 16, 84), End(6, 32, 100))),
				},
			},
		},
		{
			name:     "import disjoint directory with unexpected terramate block",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "/stack/cfg.tm",
					body: `import {
						source = "/other/cfg.tm"
					}`,
				},
				{
					filename: "/other/cfg.tm",
					body: `terramate {
						required_version = "1.0"
					}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrUnexpectedTerramate),
				},
			},
		},
		{
			name:     "import disjoint directory",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "/stack/cfg.tm",
					body: `import {
						source = "/other/cfg.tm"
					}`,
				},
				{
					filename: "/other/cfg.tm",
					body: `globals {
						A = 1
					}`,
				},
			},
			want: want{
				config: hcl.Config{},
			},
		},
		{
			name:     "import relative directory",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "/stack/cfg.tm",
					body: `import {
						source = "../other/cfg.tm"
					}`,
				},
				{
					filename: "/other/cfg.tm",
					body:     `globals {}`,
				},
			},
			want: want{
				config: hcl.Config{},
			},
		},
		{
			name:     "import relative directory outside terramate root",
			parsedir: "project/stack",
			rootdir:  "project",
			input: []cfgfile{
				{
					filename: "/project/stack/cfg.tm",
					body: `import {
						source = "../../outside/cfg.tm"
					}`,
				},
				{
					filename: "/outside/cfg.tm",
					body:     `globals {}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("project/stack/cfg.tm", Start(2, 16, 24), End(2, 38, 46))),
				},
			},
		},
		{
			name:     "import with redefinition of top-level attributes",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/imported.tm"
					}
					A = "test"
					`,
				},
				{
					filename: "other/imported.tm",
					body:     `A = "test"`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("stack/cfg.tm", Start(4, 6, 57), End(4, 7, 58))),
				},
			},
		},
		{
			name:     "import stacks - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/cfg.tm"
					}`,
				},
				{
					filename: "other/cfg.tm",
					body:     `stack {}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("stack/cfg.tm", Start(2, 16, 24), End(2, 31, 39))),
				},
			},
		},
		{
			name: "invalid import glob pattern",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     `import { source = "../imports/[*" }`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("cfg.tm", Start(1, 19, 18), End(1, 34, 33))),
				},
			},
		},
		{
			name: "import glob with no matching files",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body:     `import { source = "../imports/*" }`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						Mkrange("cfg.tm", Start(1, 19, 18), End(1, 33, 32))),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
