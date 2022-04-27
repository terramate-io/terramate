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
	"github.com/mineiros-io/terramate/generate/genfile"
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

	generateFile := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_file", builders...)
	}

	labels := func(labels ...string) hclwrite.BlockBuilder {
		return hclwrite.Labels(labels...)
	}

	//expr := func(name string, expr string) hclwrite.BlockBuilder {
	//return hclwrite.Expression(name, expr)
	//}

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
