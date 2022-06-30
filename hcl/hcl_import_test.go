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
		/*{
			name: "import missing source - fails",
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
						mkrange("cfg.tm", start(1, 8, 7), end(1, 8, 18))),
				},
			},
		},*/
		{
			name: "import with non-existent file - fails",
			dir:  "stack",
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
					errors.E(hcl.ErrImportCycle,
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
					errors.E(hcl.ErrImportCycle,
						mkrange("cfg.tm", start(2, 16, 24), end(2, 26, 34))),
				},
			},
		},
		{
			name: "import disjoint directory",
			dir:  "stack",
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
			name: "import disjoint directory with sub blocks",
			dir:  "stack",
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
								DefaultBranch: "main",
							},
						},
					},
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
