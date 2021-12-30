// Copyright 2021 Mineiros GmbH
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

package terramate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestExportAsLocals(t *testing.T) {
	type (
		block struct {
			path string
			add  *hclwrite.Block
		}
		testcase struct {
			name    string
			layout  []string
			blocks  []block
			want    map[string]*hclwrite.Block
			wantErr bool
		}
	)

	// Usually in Go names are cammel case, but on this case
	// we want it to be as close to original HCL as possible (DSL).
	export_as_locals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.NewBuilder("globals", builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.NewBuilder("globals", builders...)
	}
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name:   "no stacks no exported locals",
			layout: []string{},
		},
		{
			name:   "single stacks no no exported locals",
			layout: []string{"s:stack"},
		},
		{
			name: "two stacks no exported locals",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
		},
		{
			name:   "single stack with its own exported locals using own globals",
			layout: []string{"s:stack"},
			blocks: []block{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: export_as_locals(
						expr("string_local", "global.some_string"),
						expr("number_local", "global.some_number"),
						expr("bool_local", "global.some_bool"),
					),
				},
			},
			want: map[string]*hclwrite.Block{
				"/stack": export_as_locals(
					str("string_local", "string"),
					number("number_local", 777),
					boolean("bool_local", true),
				),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Skip()

			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, block := range tcase.blocks {
				path := filepath.Join(s.RootDir(), block.path)
				test.AppendFile(t, path, config.Filename, block.add.String())
			}

			wantExportAsLocals := tcase.want

			metadata := s.LoadMetadata()
			for _, stackMetadata := range metadata.Stacks {

				globals := s.LoadStackGlobals(stackMetadata)
				got, err := terramate.LoadStackExportAsLocals(
					s.RootDir(),
					stackMetadata,
					globals,
				)

				if tcase.wantErr {
					assert.Error(t, err)
					continue
				}

				assert.NoError(t, err)

				want, ok := wantExportAsLocals[stackMetadata.Path]
				if !ok {
					want = export_as_locals()
				}
				delete(wantExportAsLocals, stackMetadata.Path)

				if want.HasExpressions() {
					t.Errorf("wanted export_as_locals definition:\n%s\n", want)
					t.Fatal("can't contain expressions, loaded export_as_locals are evaluated (values only)")
				}

				gotAttrs := got.Attributes()
				wantAttrs := want.AttributesValues()

				if len(gotAttrs) != len(wantAttrs) {
					t.Errorf("got %d global attributes; wanted %d", len(gotAttrs), len(wantAttrs))
				}

				for name, wantVal := range wantAttrs {
					gotVal, ok := gotAttrs[name]
					if !ok {
						t.Errorf("wanted global.%s is missing", name)
						continue
					}
					if !gotVal.RawEquals(wantVal) {
						t.Errorf("got global.%s=%v; want %v", name, gotVal, wantVal)
					}
				}
			}

			if len(wantExportAsLocals) > 0 {
				t.Fatalf("wanted stack export as locals: %v that was not found on stacks: %v", wantExportAsLocals, metadata.Stacks)
			}
		})
	}
}

func TestLoadStackExportAsLocalsErrorOnRelativeDir(t *testing.T) {
	s := sandbox.New(t)
	s.BuildTree([]string{"s:stack"})

	rel, err := filepath.Rel(test.Getwd(t), s.RootDir())
	assert.NoError(t, err)

	meta := s.LoadMetadata()
	assert.EqualInts(t, 1, len(meta.Stacks))

	stackMetadata := meta.Stacks[0]
	globals := s.LoadStackGlobals(stackMetadata)
	exportLocals, err := terramate.LoadStackExportAsLocals(rel, stackMetadata, globals)
	assert.Error(t, err, "got %v instead of error", exportLocals)
}
