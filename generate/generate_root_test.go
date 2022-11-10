package generate_test

import (
	"fmt"
	"testing"

	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"

	"github.com/mineiros-io/terramate/generate"
)

func TestGenerateRootContext(t *testing.T) {
	testCodeGeneration(t, []testcase{
		{
			name: "empty generates empty file",
			configs: []hclconfig{
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/empty.txt"),
						Expr("context", "root"),
						Str("content", ""),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"empty.txt": stringer(""),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/target",
						Created: []string{"empty.txt"},
					},
				},
			},
		},
		{
			name: "generate_file with false condition generates nothing",
			configs: []hclconfig{
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "terramate.stacks.list with root workdir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclconfig{
				{
					path: "/source",
					add: Doc(
						GenerateFile(
							Labels("/stacks.txt"),
							Expr("context", "root"),
							Expr("content", `"${tm_jsonencode(terramate.stacks.list)}"`),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"stacks.txt": stringer(`["/stacks/stack-1","/stacks/stack-2"]`),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/",
						Created: []string{"stacks.txt"},
					},
				},
			},
		},
		{
			name: "generate files for all directories",
			configs: []hclconfig{
				{
					path: "/",
					add: Doc(
						GenerateFile(
							Labels("/file1.txt"),
							Expr("context", "root"),
							Str("content", "/file1.txt"),
						),
						GenerateFile(
							Labels("/file2.txt"),
							Expr("context", "root"),
							Str("content", "/file2.txt"),
						),
					),
				},
				{
					path: "/nested/dir",
					add: Doc(
						GenerateFile(
							Labels("/target/dir/file1.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file1.txt"),
						),
						GenerateFile(
							Labels("/target/dir/file2.txt"),
							Expr("context", "root"),
							Str("content", "/target/dir/file2.txt"),
						),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/file1.txt"),
						"file2.txt": stringer("/file2.txt"),
					},
				},
				{
					dir: "/target/dir",
					files: map[string]fmt.Stringer{
						"file1.txt": stringer("/target/dir/file1.txt"),
						"file2.txt": stringer("/target/dir/file2.txt"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/",
						Created: []string{"file1.txt", "file2.txt"},
					},
					{
						Dir:     "/target/dir",
						Created: []string{"file1.txt", "file2.txt"},
					},
				},
			},
		},
		{
			name: "generate_file.context=stack is ignored if not parent of any stack",
			configs: []hclconfig{
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content"),
					),
				},
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "stack"),
						Str("content", "content"),
					),
				},
			},
		},
	})
}
