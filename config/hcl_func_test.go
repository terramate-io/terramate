// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/zclconf/go-cty/cty"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/terramate-io/terramate/test"
)

func TestTmSource(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name            string
		source          string
		stackDir        string
		componentSource string
		bundleSource    string

		want string
	}

	tests := []testcase{
		// project path - local component
		{
			name:            "project path - local component - 1",
			source:          "/path/to/component",
			stackDir:        "/",
			componentSource: "/path/to/component",
			want:            "path/to/component",
		},
		{
			name:            "project path - local component - 2",
			source:          "/path/to/component/module",
			stackDir:        "/stack",
			componentSource: "/path/to/component",
			want:            "../path/to/component/module",
		},
		{
			name:            "project path - local component - 3",
			source:          "/path/to/modules/mod1",
			stackDir:        "/path/to/stack",
			componentSource: "/path/to/component",
			want:            "../modules/mod1",
		},
		// relative path - local component
		{
			name:            "relative - local component - 1",
			source:          ".",
			stackDir:        "/path/to/stack",
			componentSource: "/path/to/component",
			want:            "../component",
		},
		{
			name:            "relative - local component - 2",
			source:          "./module",
			stackDir:        "/stacks/stack1",
			componentSource: "/path/to/component",
			want:            "../../path/to/component/module",
		},
		{
			name:            "relative - local component - 3",
			source:          "../../../module",
			stackDir:        "/",
			componentSource: "/path/to/component",
			want:            "module",
		},
		{
			name:            "relative - local component - 4",
			source:          "../../../module",
			stackDir:        "/stack",
			componentSource: "/path/to/component",
			want:            "../module",
		},
		{
			name:            "relative - local component - 5",
			source:          "../module",
			stackDir:        "/stack",
			componentSource: "../component",
			bundleSource:    "/path/to/bundle",
			want:            "../path/to/module",
		},
		// remote component
		{
			name:            "project path - remote component",
			source:          "/path/to/modules/mod1",
			stackDir:        "/ignored",
			componentSource: "github.com/user/repo//path/to/component?ref=branch",
			want:            "github.com/user/repo//path/to/modules/mod1?ref=branch",
		},
		{
			name:            "dot - remote source",
			source:          ".",
			stackDir:        "/ignored",
			componentSource: "github.com/user/repo//path/to/component?ref=branch",
			bundleSource:    "github.com/user/ignored",
			want:            "github.com/user/repo//path/to/component?ref=branch",
		},
		// remote bundle
		{
			name:            "project path - remote source from bundle",
			source:          "/path/to/modules/mod1",
			stackDir:        "/ignored",
			componentSource: "/path/to/component",
			bundleSource:    "github.com/user/repo//path/to/bundle?ref=branch",
			want:            "github.com/user/repo//path/to/modules/mod1?ref=branch",
		},
		{
			name:            "dot - remote source from bundle",
			source:          ".",
			stackDir:        "/ignored",
			componentSource: "/path/to/component",
			bundleSource:    "github.com/user/repo//path/to/bundle?ref=branch",
			want:            "github.com/user/repo//path/to/component?ref=branch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rootdir := test.TempDir(t)
			ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
			ctx.SetFunction("tm_source", config.TmSourceFunc(tc.stackDir, tc.componentSource, tc.bundleSource))

			val, err := ctx.Eval(test.NewExpr(t, fmt.Sprintf(`tm_source("%s")`, tc.source)))
			assert.NoError(t, err)
			assert.EqualStrings(t, tc.want, val.AsString())
		})
	}
}

func TestTmBundles(t *testing.T) {
	t.Parallel()

	envProd := &config.Environment{ID: "prod-id", Name: "prod"}
	envDev := &config.Environment{ID: "dev-id", Name: "dev"}

	newBundle := func(alias, class string, env *config.Environment) *config.Bundle {
		return &config.Bundle{
			Alias:              alias,
			DefinitionMetadata: config.Metadata{Class: class},
			Environment:        env,
			Inputs:             map[string]cty.Value{},
			Exports:            map[string]cty.Value{},
		}
	}

	aliasesOf := func(t *testing.T, val cty.Value) []string {
		t.Helper()
		var out []string
		for it := val.ElementIterator(); it.Next(); {
			_, v := it.Element()
			out = append(out, v.GetAttr("alias").AsString())
		}
		return out
	}

	evalBundles := func(t *testing.T, reg *config.Registry, currentEnv *config.Environment, expr string) cty.Value {
		t.Helper()
		rootdir := test.TempDir(t)
		ctx := eval.NewContext(stdlib.Functions(rootdir, []string{}))
		ctx.SetFunction("tm_bundles", config.BundlesFunc(reg, currentEnv))

		val, err := ctx.Eval(test.NewExpr(t, expr))
		assert.NoError(t, err)
		return val
	}

	t.Run("results are sorted by alias regardless of registry order", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("charlie", "team", nil),
				newBundle("alpha", "team", nil),
				newBundle("bravo", "team", nil),
			},
		}

		got := aliasesOf(t, evalBundles(t, reg, nil, `tm_bundles("team")`))
		assert.EqualInts(t, 3, len(got))
		assert.EqualStrings(t, "alpha", got[0])
		assert.EqualStrings(t, "bravo", got[1])
		assert.EqualStrings(t, "charlie", got[2])
	})

	t.Run("sort order is class, then env ID, then alias", func(t *testing.T) {
		// Env-less bundles (env sort key "") must come before env-scoped bundles
		// within the same class. Env filter keeps only one non-empty env ID in the
		// result set, so that edge is where the env tiebreaker matters.
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("z", "team-b", envProd),
				newBundle("a", "team-b", nil),
				newBundle("b", "team-a", envProd),
				newBundle("a", "team-a", envProd),
				newBundle("c", "team-a", nil),
			},
		}

		val := evalBundles(t, reg, envProd, `tm_bundles("*")`)
		type row struct{ class, env, alias string }
		var got []row
		for it := val.ElementIterator(); it.Next(); {
			_, v := it.Element()
			envAttr := v.GetAttr("environment")
			env := ""
			if envAttr.GetAttr("available").True() {
				env = envAttr.GetAttr("id").AsString()
			}
			got = append(got, row{
				class: v.GetAttr("class").AsString(),
				env:   env,
				alias: v.GetAttr("alias").AsString(),
			})
		}
		want := []row{
			{"team-a", "", "c"},
			{"team-a", "prod-id", "a"},
			{"team-a", "prod-id", "b"},
			{"team-b", "", "a"},
			{"team-b", "prod-id", "z"},
		}
		assert.EqualInts(t, len(want), len(got))
		for i := range want {
			assert.EqualStrings(t, want[i].class, got[i].class, fmt.Sprintf("row %d class", i))
			assert.EqualStrings(t, want[i].env, got[i].env, fmt.Sprintf("row %d env", i))
			assert.EqualStrings(t, want[i].alias, got[i].alias, fmt.Sprintf("row %d alias", i))
		}
	})

	t.Run("filters by class", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("a", "team", nil),
				newBundle("b", "other", nil),
				newBundle("c", "team", nil),
			},
		}

		got := aliasesOf(t, evalBundles(t, reg, nil, `tm_bundles("team")`))
		assert.EqualInts(t, 2, len(got))
		assert.EqualStrings(t, "a", got[0])
		assert.EqualStrings(t, "c", got[1])
	})

	t.Run("wildcard class matches all", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("b", "other", nil),
				newBundle("a", "team", nil),
			},
		}

		got := aliasesOf(t, evalBundles(t, reg, nil, `tm_bundles("*")`))
		// Sorted by class first: "other" < "team".
		assert.EqualInts(t, 2, len(got))
		assert.EqualStrings(t, "b", got[0])
		assert.EqualStrings(t, "a", got[1])
	})

	t.Run("filters by current environment", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("a", "team", envProd),
				newBundle("b", "team", envDev),
				newBundle("c", "team", nil), // env-less bundles are always included
			},
		}

		got := aliasesOf(t, evalBundles(t, reg, envProd, `tm_bundles("team")`))
		// Env-less (sort key "") comes before env-scoped ("prod-id").
		assert.EqualInts(t, 2, len(got))
		assert.EqualStrings(t, "c", got[0])
		assert.EqualStrings(t, "a", got[1])
	})

	t.Run("explicit env arg overrides current environment", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("a", "team", envProd),
				newBundle("b", "team", envDev),
			},
		}

		got := aliasesOf(t, evalBundles(t, reg, envProd, `tm_bundles("team", "dev-id")`))
		assert.EqualInts(t, 1, len(got))
		assert.EqualStrings(t, "b", got[0])
	})

	t.Run("empty registry returns empty tuple", func(t *testing.T) {
		got := aliasesOf(t, evalBundles(t, &config.Registry{}, nil, `tm_bundles("team")`))
		assert.EqualInts(t, 0, len(got))
	})

	t.Run("no class match returns empty tuple", func(t *testing.T) {
		reg := &config.Registry{
			Bundles: []*config.Bundle{
				newBundle("a", "other", nil),
			},
		}
		got := aliasesOf(t, evalBundles(t, reg, nil, `tm_bundles("team")`))
		assert.EqualInts(t, 0, len(got))
	})
}
