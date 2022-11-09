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
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test"
	errtest "github.com/mineiros-io/terramate/test/errors"
	infotest "github.com/mineiros-io/terramate/test/hclutils/info"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGenerateFiles(t *testing.T) {
	t.Parallel()

	tcases := []testcase{

		{
			name:  "using lets and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("data", "let-data"),
						),
						Str("content", "${let.data}-${terramate.stack.path.absolute}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "let-data-/stack",
					},
				},
			},
		},
		{
			name:  "using lets, globals and metadata with interpolation",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/globals.tm",
					add: Globals(
						Str("string", "global string"),
					),
				},
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Expr("string", `global.string`),
							Expr("path", `terramate.stack.path.absolute`),
						),
						Str("content", "${let.string}-${let.path}"),
					),
				},
			},
			want: []result{
				{
					name: "test",
					file: genFile{
						condition: true,
						body:      "global string-/stack",
					},
				},
			},
		},
		{
			name:  "generate_file with duplicated lets attrs",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: GenerateFile(
						Labels("test"),
						Lets(
							Str("string", "let string"),
						),
						Lets(
							Str("string", "dup"),
						),
						Expr("content", `let.string`),
					),
				},
			},
			wantErr: errors.E(lets.ErrRedefined),
		},
		{
			name:  "lets are scoped",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack/test.tm",
					add: Doc(
						GenerateFile(
							Labels("test"),
							Lets(
								Str("some_str", "test"),
							),
							Expr("content", `let.some_str`),
						),
						GenerateFile(
							Labels("test2"),
							Expr("content", `let.some_str`),
						),
					),
				},
			},
			wantErr: errors.E(genfile.ErrContentEval),
		},
	}

	for _, tcase := range tcases {
		testGenfile(t, tcase)
	}
}

type (
	hclconfig struct {
		path string
		add  fmt.Stringer
	}
	genFile struct {
		origin    info.Range
		body      string
		condition bool
		asserts   []config.Assert
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

func testGenfile(t *testing.T, tcase testcase) {
	t.Run(tcase.name, func(t *testing.T) {
		t.Parallel()

		s := sandbox.New(t)
		s.BuildTree([]string{"s:" + tcase.stack})
		stacks := s.LoadStacks()
		projmeta := stack.NewProjectMetadata(s.RootDir(), stacks)
		stack := s.LoadStacks()[0]

		for _, cfg := range tcase.configs {
			test.AppendFile(t, s.RootDir(), cfg.path, cfg.add.String())
		}

		cfg, err := config.LoadTree(s.RootDir(), s.RootDir())
		if errors.IsAnyKind(tcase.wantErr, hcl.ErrHCLSyntax, hcl.ErrTerramateSchema) {
			errtest.Assert(t, err, tcase.wantErr)
			return
		}

		assert.NoError(t, err)

		globals := s.LoadStackGlobals(cfg, projmeta, stack)
		got, err := genfile.Load(cfg, projmeta, stack, globals)
		errtest.Assert(t, err, tcase.wantErr)

		if len(got) != len(tcase.want) {
			for i, file := range got {
				t.Logf("got[%d] = %+v", i, file)
			}
			for i, file := range tcase.want {
				t.Logf("want[%d] = %+v", i, file)
			}
			t.Fatalf("length of got and want mismatch: got %d but want %d",
				len(got), len(tcase.want))
		}

		for i, want := range tcase.want {
			gotfile := got[i]
			gotbody := gotfile.Body()
			wantbody := want.file.body

			if gotfile.Condition() != want.file.condition {
				t.Fatalf("got condition %t != wanted %t", gotfile.Condition(), want.file.condition)
			}

			want.file.origin = infotest.FixRange(s.RootDir(), want.file.origin)

			test.AssertEqualRanges(t, gotfile.Range(), want.file.origin, "block range")

			test.FixupRangeOnAsserts(s.RootDir(), want.file.asserts)
			test.AssertConfigEquals(t, gotfile.Asserts(), want.file.asserts)

			assert.EqualStrings(t,
				want.name,
				gotfile.Label(),
				"wrong name config path for generated code",
			)

			assert.EqualStrings(t, wantbody, gotbody,
				"generated file body differs",
			)
		}
	})
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
