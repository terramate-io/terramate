// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"encoding/json"
	"testing"

	. "github.com/terramate-io/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestListJSON(t *testing.T) {
	t.Parallel()

	type stackInfo struct {
		Path         string   `json:"path"`
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Tags         []string `json:"tags"`
		Dependencies []string `json:"dependencies"`
		Reason       string   `json:"reason"`
		IsChanged    bool     `json:"is_changed"`
	}

	type testcase struct {
		name   string
		layout []string
		want   map[string][]string // map of stack path -> dependencies
	}

	for _, tcase := range []testcase{
		{
			name: "simple dependency: stack1 after stack2",
			layout: []string{
				`s:stack1:after=["/stack2"]`,
				"s:stack2",
			},
			want: map[string][]string{
				"stack1": {"stack2"},
				"stack2": {},
			},
		},
		{
			name: "linear chain: A -> B -> C",
			layout: []string{
				"s:A",
				`s:B:after=["/A"]`,
				`s:C:after=["/B"]`,
			},
			want: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"B"},
			},
		},
		{
			name: "diamond dependency",
			layout: []string{
				"s:A",
				`s:B:after=["/A"]`,
				`s:C:after=["/A"]`,
				`s:D:after=["/B","/C"]`,
			},
			want: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
		},
		{
			name: "fork-fork-join-join",
			layout: []string{
				"s:A",
				`s:B:after=["/A"]`,
				`s:C:after=["/B"]`,
				`s:D:after=["/C","/X"]`,
				`s:E:after=["/A"]`,
				`s:F:after=["/E"]`,
				`s:G:after=["/F","/Y"]`,
				`s:X:after=["/B"]`,
				`s:Y:after=["/E"]`,
				`s:Z:after=["/D","/G"]`,
			},
			want: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"B"},
				"D": {"C", "X"},
				"E": {"A"},
				"F": {"E"},
				"G": {"F", "Y"},
				"X": {"B"},
				"Y": {"E"},
				"Z": {"D", "G"},
			},
		},
		{
			name: "no dependencies",
			layout: []string{
				"s:stack1",
				"s:stack2",
				"s:stack3",
			},
			want: map[string][]string{
				"stack1": {},
				"stack2": {},
				"stack3": {},
			},
		},
		{
			name: "before statement: stack1 before stack2",
			layout: []string{
				`s:stack1:before=["/stack2"]`,
				"s:stack2",
			},
			want: map[string][]string{
				"stack1": {},
				"stack2": {"stack1"},
			},
		},
		{
			name: "mixed before and after statements",
			layout: []string{
				`s:A:before=["/B"]`,
				`s:B:after=["/C"]`,
				"s:C",
			},
			want: map[string][]string{
				"A": {},
				"B": {"A", "C"},
				"C": {},
			},
		},
		{
			name: "complex before statements",
			layout: []string{
				`s:A:before=["/B", "/C"]`,
				"s:B",
				"s:C",
				`s:D:after=["/B", "/C"]`,
			},
			want: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
		},
	} {
		tc := tcase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := sandbox.New(t)
			s.BuildTree(tc.layout)

			cli := NewCLI(t, s.RootDir())
			result := cli.Run("list", "--run-order", "--format=json")

			if result.Status != 0 {
				t.Fatalf("command failed with status %d: %s", result.Status, result.Stderr)
			}

			var got map[string]stackInfo
			if err := json.Unmarshal([]byte(result.Stdout), &got); err != nil {
				t.Fatalf("failed to unmarshal JSON output: %v\nOutput: %s", err, result.Stdout)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("expected %d stacks, got %d", len(tc.want), len(got))
			}

			for stackPath, wantDeps := range tc.want {
				info, ok := got[stackPath]
				if !ok {
					t.Fatalf("stack %q not found in output", stackPath)
				}

				if info.Path != stackPath {
					t.Fatalf("stack path mismatch: expected %q, got %q", stackPath, info.Path)
				}

				// convert to maps, order doesn't matter for dependencies
				wantDepsMap := make(map[string]bool)
				for _, dep := range wantDeps {
					wantDepsMap[dep] = true
				}

				gotDepsMap := make(map[string]bool)
				for _, dep := range info.Dependencies {
					gotDepsMap[dep] = true
				}

				if len(wantDepsMap) != len(gotDepsMap) {
					t.Fatalf("stack %q: expected %d dependencies, got %d", stackPath, len(wantDeps), len(info.Dependencies))
				}

				for dep := range wantDepsMap {
					if !gotDepsMap[dep] {
						t.Fatalf("stack %q: missing dependency %q", stackPath, dep)
					}
				}
			}
		})
	}
}
