// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
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
