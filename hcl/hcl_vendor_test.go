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
	. "github.com/mineiros-io/terramate/test/hclutils"
)

func TestHCLParserVendor(t *testing.T) {
	for _, tc := range []testcase{
		{
			name: "empty vendor",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Vendor: &hcl.VendorConfig{},
				},
			},
		},
		{
			name: "vendor.dir",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  dir = "/dir"
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Vendor: &hcl.VendorConfig{
						Dir: "/dir",
					},
				},
			},
		},
		{
			name: "empty manifest",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Vendor: &hcl.VendorConfig{
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
						vendor {
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
					Vendor: &hcl.VendorConfig{
						Manifest: &hcl.ManifestConfig{
							Default: &hcl.ManifestDesc{},
						},
					},
				},
			},
		},
		{
			name: "default has files",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						    default {
						      files = ["/", "/test"]
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Vendor: &hcl.VendorConfig{
						Manifest: &hcl.ManifestConfig{
							Default: &hcl.ManifestDesc{
								Files: []string{"/", "/test"},
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
						vendor {
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
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 13, 67), End(5, 18, 72)),
					),
				},
			},
		},
		{
			name: "vendor.dir is not string fails",
			input: []cfgfile{
				{
					filename: "vendor.tm",
					body: `
						vendor {
						  dir = ["/dir"]
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("vendor.tm", Start(3, 9, 24), End(3, 12, 27)),
					),
				},
			},
		},
		{
			name: "vendor.dir is undefined fails",
			input: []cfgfile{
				{
					filename: "vendor.tm",
					body: `
						vendor {
						  dir = undefined
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("vendor.tm", Start(3, 9, 24), End(3, 12, 27)),
					),
				},
			},
		},
		{
			name: "redefined vendor fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						}
						vendor {
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(4, 7, 30), End(4, 13, 36)),
					),
				},
			},
		},
		{
			name: "redefined manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						  }
						  manifest {
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 9, 53), End(5, 17, 61)),
					),
				},
			},
		},
		{
			name: "redefined default fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						    default {
						    }
						    default {
						    }
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(6, 11, 77), End(6, 18, 84)),
					),
				},
			},
		},
		{
			name: "files is not string list fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
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
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 13, 67), End(5, 18, 72)),
					),
				},
			},
		},
		{
			name: "files has undefined element fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						    default {
						      files = ["ok", ns.undefined]
						    }
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 28, 82), End(5, 30, 84)),
					),
				},
			},
		},
		{
			name: "files is undefined fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						    default {
						      files = local.files
						    }
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 21, 75), End(5, 26, 80)),
					),
				},
			},
		},
		{
			name: "unrecognized attribute on vendor fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						    unknown = true
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(3, 11, 26), End(3, 18, 33)),
					),
				},
			},
		},
		{
			name: "unrecognized attribute on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest {
						    unknown = true
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(4, 11, 45), End(4, 18, 52)),
					),
				},
			},
		},
		{
			name: "label on vendor fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor "label" {
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(2, 14, 14), End(2, 21, 21)),
					),
				},
			},
		},
		{
			name: "label on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  manifest "label" {
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(3, 18, 33), End(3, 25, 40)),
					),
				},
			},
		},
		{
			name: "unrecognized block on vendor fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
						  unknown {
						  }
						}
					`,
				},
			},
			want: want{
				errs: []error{
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(3, 9, 24), End(3, 16, 31)),
					),
				},
			},
		},
		{
			name: "unrecognized block on manifest fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
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
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(4, 11, 45), End(4, 18, 52)),
					),
				},
			},
		},
		{
			name: "unrecognized attribute on default fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
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
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 13, 67), End(5, 20, 74)),
					),
				},
			},
		},
		{
			name: "unrecognized block on default fails",
			input: []cfgfile{
				{
					filename: "manifest.tm",
					body: `
						vendor {
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
					errors.E(hcl.ErrTerramateSchema,
						Mkrange("manifest.tm", Start(5, 13, 67), End(5, 20, 74)),
					),
				},
			},
		},
	} {
		testParser(t, tc)
	}
}
