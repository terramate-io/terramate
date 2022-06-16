package hcl_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
)

func TestHCLParserConfigRun(t *testing.T) {

	runEnvCfg := func(attrs hclsyntax.Attributes) hcl.Config {
		return hcl.Config{
			Terramate: &hcl.Terramate{
				RootConfig: &hcl.RootConfig{
					Run: &hcl.RunConfig{
						Env: &hcl.RunEnv{
							Attributes: attrs,
						},
					},
				},
			},
		}
	}

	attribute := func(expr string) *hclsyntax.Attribute {
		const attrname = "a"

		cfg := fmt.Sprintf("%s = %s", attrname, expr)
		parser := hclparse.NewParser()
		res, diags := parser.ParseHCL([]byte(cfg), "hcl-tests")
		if diags.HasErrors() {
			t.Fatalf("test case has invalid expression %q as HCL attribute", expr)
		}

		body := res.Body.(*hclsyntax.Body)
		attrs := body.Attributes
		attr, ok := attrs[attrname]
		if !ok {
			t.Fatalf("expected attribute %s to exist, got: %v", attrname, attrs)
		}

		return attr
	}

	for _, tc := range []testcase{
		{
			name: "empty run",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `terramate {
					  config {
					    run {
					    }
					  }
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Run: &hcl.RunConfig{},
						},
					},
				},
			},
		},
		{
			name: "empty run.env",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `terramate {
					  config {
					    run {
					      env {
					      }
					    }
					  }
					}`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Run: &hcl.RunConfig{
								Env: &hcl.RunEnv{},
							},
						},
					},
				},
			},
		},
		{
			name: "unrecognized attribute on run",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {
						      something = "bleh"
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
			name: "unrecognized block on run",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {
						      something {
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
		{
			name: "unrecognized label on run",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run "invalid" {
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
			name: "unrecognized label on run.env",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {
						      env "invalid" {
						        something = "bleh"
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
		{
			name: "multiple empty run blocks on same config",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {}
						    run {}
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Run: &hcl.RunConfig{},
						},
					},
				},
			},
		},
		{
			name: "multiple empty run blocks on multiple config",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {}
						  }
						  config {
						    run {}
						  }
						}
					`,
				},
			},
			want: want{
				config: hcl.Config{
					Terramate: &hcl.Terramate{
						RootConfig: &hcl.RootConfig{
							Run: &hcl.RunConfig{},
						},
					},
				},
			},
		},
		{
			name: "one attr on run.env",
			input: []cfgfile{
				{
					filename: "cfg.tm",
					body: `
						terramate {
						  config {
						    run {
						      env {
						        string = "value"
						        number = 666
						        list = []
						        interp = "${global.a}"
						        traversal = global.a.b
						      }
						    }
						  }
						}
					`,
				},
			},
			want: want{
				config: runEnvCfg(hclsyntax.Attributes{
					"string":    attribute(`"value"`),
					"number":    attribute("666"),
					"list":      attribute("[]"),
					"interp":    attribute(`"${global.a}"`),
					"traversal": attribute("global.a"),
				}),
			},
		},
	} {
		testParser(t, tc)
	}
}
