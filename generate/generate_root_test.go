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

package generate_test

import (
	"fmt"
	"testing"

	"github.com/mineiros-io/terramate/errors"
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
			name: "generate.context=root has access to project metadata",
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
						Labels("file.txt"),
						Expr("context", "stack"),
						Str("content", "content"),
					),
				},
			},
		},
		{
			name: "mixing generate_file.context",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Str("content", "content"),
					),
				},
				{
					path: "/",
					add: GenerateFile(
						Labels("file.txt"),
						Str("content", "content"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/stack",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content"),
					},
				},
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/stack",
						Created: []string{"file.txt"},
					},
					{
						Dir:     "/target",
						Created: []string{"file.txt"},
					},
				},
			},
		},
		{
			name: "generate.context=root is disallowed to generate inside stacks",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/",
					add: GenerateFile(
						Labels("/stack/file.txt"),
						Str("content", "test"),
						Expr("context", "root"),
					),
				},
			},
			wantReport: generate.Report{
				Failures: []generate.FailureResult{
					{
						Result: generate.Result{
							Dir: "/stack",
						},
						Error: errors.E(generate.ErrInvalidGenBlockLabel),
					},
				},
			},
		},
		{
			name: "generate.context=root inside stack generating elsewhere",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/file.tm",
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
			name: "generate.context=root inside a subdir of stack generating elsewhere",
			layout: []string{
				"s:stack",
			},
			configs: []hclconfig{
				{
					path: "/stack/subdir/dir/file.tm",
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
			name: "two generate_file with same labels and different condition",
			configs: []hclconfig{
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", false),
						Str("content", "content 1"),
					),
				},
				{
					path: "/source/file.tm",
					add: GenerateFile(
						Labels("/target/file.txt"),
						Expr("context", "root"),
						Bool("condition", true),
						Str("content", "content 2"),
					),
				},
			},
			want: []generatedFile{
				{
					dir: "/target",
					files: map[string]fmt.Stringer{
						"file.txt": stringer("content 2"),
					},
				},
			},
			wantReport: generate.Report{
				Successes: []generate.Result{
					{
						Dir:     "/target",
						Created: []string{"file.txt"},
					},
				},
			},
		},
	})
}
