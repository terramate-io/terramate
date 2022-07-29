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

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
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
						mkrange("cfg.tm", start(1, 8, 7), end(1, 19, 18))),
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
						mkrange("cfg.tm", start(1, 8, 7), end(1, 8, 7))),
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
						mkrange("stack/cfg.tm", start(2, 16, 24), end(2, 42, 50))),
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
						mkrange("cfg.tm", start(2, 16, 24), end(2, 24, 32))),
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
						mkrange("cfg.tm", start(2, 16, 24), end(2, 26, 34))),
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
						mkrange("other/cfg.tm", start(2, 16, 24), end(2, 31, 39))),
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
						mkrange("stack/cfg.tm", start(2, 16, 24), end(2, 25, 33))),
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
						mkrange("stack/cfg.tm", start(7, 17, 92), end(7, 32, 107))),
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
						mkrange("other/cfg.tm", start(6, 16, 84), end(6, 32, 100))),
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
					body: `terramate {
						required_version = "1.0"
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "1.0",
					},
				},
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
					body: `terramate {
						required_version = "1.0"
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "1.0",
					},
				},
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
					body: `terramate {
						required_version = "1.0"
					}`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						mkrange("project/stack/cfg.tm", start(2, 16, 24), end(2, 38, 46))),
				},
			},
		},
		{
			name:     "import disjoint directory with config sub blocks",
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
							config {
								git {
									default_branch = "main"
								}
							}
						}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RequiredVersion: "1.0",
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:    "main",
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
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
						mkrange("stack/cfg.tm", start(4, 6, 57), end(4, 7, 58))),
				},
			},
		},
		{
			name:     "import with conflicting terramate blocks - fails",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/imported.tm"
					}
					terramate {
						config {
							git {
								default_branch = "main"
							}
						}
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `terramate {
						config {
							git {
								default_branch = "main"
							}
						}
					}
				`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrImport,
						mkrange("stack/cfg.tm", start(7, 9, 105), end(7, 23, 119))),
				},
			},
		},
		{
			name:     "import with merged terramate config",
			parsedir: "stack",
			input: []cfgfile{
				{
					filename: "stack/cfg.tm",
					body: `import {
						source = "/other/imported.tm"
					}
					terramate {
						config {
							git {
								default_branch = "main"
							}
						}
					}
					`,
				},
				{
					filename: "other/imported.tm",
					body: `terramate {
						config {
							git {
								default_remote = "origin"
							}
						}
					}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Config: &hcl.RootConfig{
							Git: &hcl.GitConfig{
								DefaultBranch:    "main",
								DefaultRemote:    "origin",
								CheckUntracked:   true,
								CheckUncommitted: true,
								CheckRemote:      true,
							},
						},
					},
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
						mkrange("stack/cfg.tm", start(2, 16, 24), end(2, 31, 39))),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
