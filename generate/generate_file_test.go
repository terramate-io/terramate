package generate_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate"
)

func TestGenerateFile(t *testing.T) {
	checkGenFiles := func(t *testing.T, got string, want string) {
		t.Helper()
		// Terramate header validation is done separately, here we check only code.
		// So headers are removed.
		got = removeTerramateHCLHeader(got)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Error("generated code doesn't match expectation")
			t.Errorf("want:\n%q", want)
			t.Errorf("got:\n%q", got)
			t.Fatalf("diff:\n%s", diff)
		}
	}
	testCodeGeneration(t, checkGenFiles, []testcase{
		{
			name: "empty generate_file content generates nothing",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: generateFile(
						labels("empty"),
						strAttr("content", ""),
					),
				},
			},
		},
		{
			name: "generate files for all stacks from parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks",
					add: hcldoc(
						generateFile(
							labels("file1.txt"),
							exprAttr("content", "terramate.path"),
						),
						generateFile(
							labels("file2.txt"),
							exprAttr("content", "terramate.name"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack-1",
					files: map[string]fmt.Stringer{
						"file1.txt": str("/stacks/stack-1"),
						"file2.txt": str("stack-1"),
					},
				},
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": str("/stacks/stack-2"),
						"file2.txt": str("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						StackPath: "/stacks/stack-1",
						Created:   []string{"file1.txt", "file2.txt"},
					},
					{
						StackPath: "/stacks/stack-2",
						Created:   []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			name: "generate files for single stack",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-2",
					add: hcldoc(
						generateFile(
							labels("file1.txt"),
							exprAttr("content", "terramate.path"),
						),
						generateFile(
							labels("file2.txt"),
							exprAttr("content", "terramate.name"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					stack: "/stacks/stack-2",
					files: map[string]fmt.Stringer{
						"file1.txt": str("/stacks/stack-2"),
						"file2.txt": str("stack-2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						StackPath: "/stacks/stack-2",
						Created:   []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			// TODO(katcipis): define a proper behavior where
			// directories are allowed but in a constrained fashion.
			// This is a quick fix to avoid creating files on arbitrary
			// places around the file system.
			name: "generate file with dir separators on label name fails",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
				"s:stacks/stack-4",
			},
			configs: []hclconfig{
				{
					path: "/stacks/stack-1",
					add: generateFile(
						labels("/name"),
						strAttr("content", "something"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: generateFile(
						labels("./name"),
						strAttr("content", "something"),
					),
				},
				{
					path: "/stacks/stack-3",
					add: generateFile(
						labels("./dir/name"),
						strAttr("content", "something"),
					),
				},
				{
					path: "/stacks/stack-4",
					add: generateFile(
						labels("dir/name"),
						strAttr("content", "something"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-1",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-2",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-3",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
					{
						Result: generate.Result{
							StackPath: "/stacks/stack-4",
						},
						Error: errors.E(generate.ErrInvalidFilePath),
					},
				},
			},
		},
	})
}
