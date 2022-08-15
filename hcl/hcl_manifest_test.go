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

func TestHCLParserManifest(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "empty manifest",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Manifest: &hcl.ManifestConfig{},
					},
				},
			},
		},
		{
			name: "empty manifest.default",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Manifest: &hcl.ManifestConfig{
							Default: &hcl.ManifestDesc{},
						},
					},
				},
			},
		},
		{
			name: "default has files and excludes",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      files = ["/", "/test"]
						      excludes = ["/test2"]
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						Manifest: &hcl.ManifestConfig{
							Default: &hcl.ManifestDesc{
								Files:    []string{"/", "/test"},
								Excludes: []string{"/test2"},
							},
						},
					},
				},
			},
		},
		{
			name: "files is not list fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      files = "not list"
						    }
						  }
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
			name: "redefined on same file fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      files = []
						    }
						  }
						}
						terramate {
						  manifest {
						    default {
						      excludes = []
						    }
						  }
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
			name: "redefined on different file fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      files = []
						    }
						  }
						}
					`,
				},
				{
					filename: "manifest2.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      excludes = []
						    }
						  }
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
			name: "files is not string list fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      files = ["ok", 666]
						    }
						  }
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
			name: "excludes is not list fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      excludes = "not list"
						    }
						  }
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
			name: "excludes is not string list fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      excludes = ["ok", true]
						    }
						  }
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
			name: "unrecognized attribute on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    unknown = true
						  }
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
			name: "label on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest "label" {
						  }
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
			name: "unrecognized block on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    unknown {
						    }
						  }
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
			name: "unrecognized attribute on default fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      unknown = true
						    }
						  }
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
			name: "unrecognized block on default fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						terramate {
						  manifest {
						    default {
						      unknown {
						      }
						    }
						  }
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
	} {
		testParser(t, tc)
	}
}
