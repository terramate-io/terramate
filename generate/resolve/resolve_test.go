// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/generate/resolve"
)

func TestCombineSources(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src       string
		parentSrc string

		want string
	}

	tests := []testcase{
		// rel + abs
		{
			src:       ".",
			parentSrc: "/parent",
			want:      "/parent",
		},
		{
			src:       "..",
			parentSrc: "/parent",
			want:      "/",
		},
		{
			src:       "./src",
			parentSrc: "/parent",
			want:      "/parent/src",
		},
		{
			src:       "../src",
			parentSrc: "/parent",
			want:      "/src",
		},
		{
			// This is filesystem-like behaviour. cd .. at / stays at /.
			src:       "../../../../src",
			parentSrc: "/parent",
			want:      "/src",
		},
		// rel + rel
		{
			src:       ".",
			parentSrc: "./parent",
			want:      "./parent",
		},
		{
			src:       ".",
			parentSrc: ".",
			want:      ".",
		},
		{
			src:       "./src",
			parentSrc: "./parent",
			want:      "./parent/src",
		},
		{
			src:       "..",
			parentSrc: "..",
			want:      "../..",
		},
		{
			src:       "..",
			parentSrc: "../parent",
			want:      "..",
		},
		{
			src:       "../src",
			parentSrc: "../parent",
			want:      "../src",
		},
		{
			src:       "./src",
			parentSrc: "../parent",
			want:      "../parent/src",
		},
		// rel + url
		{
			src:       ".",
			parentSrc: "github.com/repo",
			want:      "github.com/repo",
		},
		{
			src:       "./src",
			parentSrc: "github.com/repo//parent",
			want:      "github.com/repo//parent/src",
		},
		{
			src:       "./src",
			parentSrc: "github.com/repo//parent?ref=x",
			want:      "github.com/repo//parent/src?ref=x",
		},
		{
			src:       "..",
			parentSrc: "github.com/repo//parent?ref=x",
			want:      "github.com/repo?ref=x",
		},
		{
			src:       "../src",
			parentSrc: "github.com/repo//parent?ref=x",
			want:      "github.com/repo//src?ref=x",
		},
		{
			src:       "./src",
			parentSrc: "github.com/repo",
			want:      "github.com/repo//src",
		},
		{
			src:       "../../../src",
			parentSrc: "github.com/repo",
			want:      "github.com/repo//src",
		},
		// abs + rel
		{
			src:       "/used",
			parentSrc: "./ignored",
			want:      "/used",
		},
		// abs + abs
		{
			src:       "/used",
			parentSrc: "/ignored",
			want:      "/used",
		},
		// abs + url
		{
			src:       "/src",
			parentSrc: "github.com/repo//parent",
			want:      "github.com/repo//src",
		},
		{
			src:       "/src",
			parentSrc: "github.com/repo//parent?ref=x",
			want:      "github.com/repo//src?ref=x",
		},
		{
			src:       "/src",
			parentSrc: "github.com/repo?ref=x",
			want:      "github.com/repo//src?ref=x",
		},
		// url + rel
		{
			src:       "github.com/repo//used?ref=x",
			parentSrc: "./ignored",
			want:      "github.com/repo//used?ref=x",
		},
		// url + abs
		{
			src:       "github.com/repo//used",
			parentSrc: "/ignored",
			want:      "github.com/repo//used",
		},
		// url + url
		{
			src:       "github.com/repo",
			parentSrc: "github.com/ignored",
			want:      "github.com/repo",
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s + %s", tc.src, tc.parentSrc), func(t *testing.T) {
			got := resolve.CombineSources(tc.src, tc.parentSrc)
			assert.EqualStrings(t, tc.want, got)
		})
	}
}
