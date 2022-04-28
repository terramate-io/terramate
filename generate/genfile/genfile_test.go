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

package genfile_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGenerateFiles(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		genFile struct {
			body   string
			origin string
		}
		result struct {
			name string
			file genFile
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    []result
			wantErr error
		}
	)

	hcldoc := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildHCL(builders...)
	}
	generateFile := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_file", builders...)
	}
	labels := func(labels ...string) hclwrite.BlockBuilder {
		return hclwrite.Labels(labels...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	expr := func(name string, expr string) hclwrite.BlockBuilder {
		return hclwrite.Expression(name, expr)
	}
	str := func(name string, val string) hclwrite.BlockBuilder {
		return hclwrite.String(name, val)
	}

	tcases := []testcase{
		{
			name:  "no generation",
			stack: "/stack",
		},
		{
			name:  "empty content attribute generates empty body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/empty.tm",
					add: generateFile(
						labels("empty"),
						str("content", ""),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					file: genFile{
						origin: "/stack/empty.tm",
						body:   "",
					},
				},
			},
		},
		{
			name:  "simple plain string",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "test"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						origin: "/stack/test.tm",
						body:   "test",
					},
				},
			},
		},
		{
			name:  "using globals and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test"),
						str("content", "${global.data}-${terramate.path}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						origin: "/stack/test.tm",
						body:   "global-data-/stack",
					},
				},
			},
		},
		{
			name:  "multiple generate_file blocks on same file",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test1"),
							expr("content", "global.data"),
						),
						generateFile(
							labels("test2"),
							expr("content", "terramate.path"),
						),
						generateFile(
							labels("test3"),
							str("content", "terramate!"),
						),
					),
				},
			},
			want: []result{
				{
					name: "test1",
					file: genFile{
						origin: "/stack/test.tm",
						body:   "global-data",
					},
				},
				{
					name: "test2",
					file: genFile{
						origin: "/stack/test.tm",
						body:   "/stack",
					},
				},
				{
					name: "test3",
					file: genFile{
						origin: "/stack/test.tm",
						body:   "terramate!",
					},
				},
			},
		},
		{
			name:  "using globals and metadata with functions",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stack/json.tm",
					add: generateFile(
						labels("test.json"),
						expr("content", "tm_jsonencode({field = global.data})"),
					),
				},
				{
					path: "/stack/yaml.tm",
					add: generateFile(
						labels("test.yml"),
						expr("content", "tm_yamlencode({field = terramate.path})"),
					),
				},
			},
			want: []result{
				{
					name: "test.json",
					file: genFile{
						origin: "/stack/json.tm",
						body:   `{"field":"global-data"}`,
					},
				},
				{
					name: "test.yml",
					file: genFile{
						origin: "/stack/yaml.tm",
						body:   "\"field\": \"/stack\"\n",
					},
				},
			},
		},
		{
			name:  "hierarchical load",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/root.tm",
					add: generateFile(
						labels("root"),
						str("content", "root-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/globals.tm",
					add: globals(
						str("data", "global-data"),
					),
				},
				{
					path: "/stacks/stacks.tm",
					add: generateFile(
						labels("stacks"),
						str("content", "stacks-${global.data}-${terramate.path}"),
					),
				},
				{
					path: "/stacks/stack/stack.tm",
					add: generateFile(
						labels("stack"),
						str("content", "stack-${global.data}-${terramate.path}"),
					),
				},
			},
			want: []result{
				{
					name: "root",
					file: genFile{
						origin: "/root.tm",
						body:   "root-global-data-/stacks/stack",
					},
				},
				{
					name: "stacks",
					file: genFile{
						origin: "/stacks/stacks.tm",
						body:   "stacks-global-data-/stacks/stack",
					},
				},
				{
					name: "stack",
					file: genFile{
						origin: "/stacks/stack/stack.tm",
						body:   "stack-global-data-/stacks/stack",
					},
				},
			},
		},
		{
			name:  "content must be string",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: generateFile(
						labels("test.yml"),
						expr("content", "5"),
					),
				},
			},
			wantErr: errors.E(genfile.ErrInvalidContentType),
		},
		{
			name:  "conflicting blocks on same file",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
						generateFile(
							labels("test.yml"),
							str("content", "test2"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrLabelConflict),
		},
		{
			name:  "conflicting blocks on same dir",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
					),
				},
				{
					path: "/stack/test2.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test2"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrLabelConflict),
		},
		{
			name:  "conflicting blocks on different dirs",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "root"),
						),
					),
				},
				{
					path: "/stack/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml"),
							str("content", "test"),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrLabelConflict),
		},
		{
			name:  "generate_file missing label",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file with two labels",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels("test.yml", "test"),
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
		{
			name:  "generate_file with empty label",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/test.tm",
					add: hcldoc(
						generateFile(
							labels(""),
							str("content", "root"),
						),
					),
				},
			},
			wantErr: errors.E(hcl.ErrTerramateSchema),
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				test.AppendFile(t, s.RootDir(), cfg.path, cfg.add.String())
			}

			globals := s.LoadStackGlobals(stack)
			res, err := genfile.Load(s.RootDir(), stack, globals)
			errtest.Assert(t, err, tcase.wantErr)

			got := res.GeneratedFiles()

			for _, res := range tcase.want {
				gotFile, ok := got[res.name]
				if !ok {
					t.Fatalf("want generated file %q but got none", res.name)
				}
				gotBody := gotFile.Body()
				wantBody := res.file.body

				assert.EqualStrings(t,
					res.file.origin,
					gotFile.Origin(),
					"wrong origin config path for generated code",
				)

				assert.EqualStrings(t, wantBody, gotBody,
					"generated file body differs",
				)

				delete(got, res.name)
			}

			assert.EqualInts(t, 0, len(got), "got unexpected exported code: %v", got)
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
