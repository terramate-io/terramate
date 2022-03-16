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

package generate_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLocalsGeneration(t *testing.T) {
	// The test approach for locals generation already uses a new test package
	// to help creating the HCL files instead of using plain raw strings.
	// There are some trade-offs involved and we are assessing how to approach
	// the testing, hence for now it is inconsistent between locals generation
	// and backend configuration generation. The idea is to converge to a single
	// approach ASAP.
	type (
		// hclblock represents an HCL block that will be appended on
		// the file path.
		hclblock struct {
			path string
			add  *hclwrite.Block
		}
		want struct {
			err          error
			stacksLocals map[string]*hclwrite.Block
		}
		testcase struct {
			name       string
			layout     []string
			configs    []hclblock
			workingDir string
			want       want
		}
	)

	// gen instead of generate because name conflicts with generate pkg
	gen := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate", builders...)
	}
	// cfg instead of config because name conflicts with config pkg
	cfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("config", builders...)
	}

	tcases := []testcase{
		{
			name:   "no stacks no exported locals",
			layout: []string{},
		},
		{
			name:   "single stacks no exported locals",
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
			configs: []hclblock{
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
					add: exportAsLocals(
						expr("string_local", "global.some_string"),
						expr("number_local", "global.some_number"),
						expr("bool_local", "global.some_bool"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stack": locals(
						boolean("bool_local", true),
						number("number_local", 777),
						str("string_local", "string"),
					),
				},
			},
		},
		{
			name:   "single stack exporting metadata using functions",
			layout: []string{"s:stack"},
			configs: []hclblock{
				{
					path: "/stack",
					add: exportAsLocals(
						expr("funny_path", `tm_replace(terramate.path, "/", "@")`),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stack": locals(
						str("funny_path", "@stack"),
					),
				},
			},
		},
		{
			name:   "single stack referencing undefined global fails",
			layout: []string{"s:stack"},
			configs: []hclblock{
				{
					path: "/stack",
					add: exportAsLocals(
						expr("undefined", "global.undefined"),
					),
				},
			},
			want: want{
				err: generate.ErrExportingLocalsGen,
			},
		},
		{
			name: "multiple stack with exported locals being overridden",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("attr1", "value1"),
						str("attr2", "value2"),
						str("attr3", "value3"),
					),
				},
				{
					path: "/",
					add: exportAsLocals(
						expr("string", "global.attr1"),
					),
				},
				{
					path: "/stacks",
					add: exportAsLocals(
						expr("string", "global.attr2"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: exportAsLocals(
						expr("string", "global.attr3"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-1": locals(
						str("string", "value3"),
					),
					"/stacks/stack-2": locals(
						str("string", "value2"),
					),
				},
			},
		},
		{
			name:   "single stack with exported locals and globals from parent dirs",
			layout: []string{"s:stacks/stack"},
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/",
					add: exportAsLocals(
						expr("num_local", "global.num"),
						expr("path_local", "terramate.path"),
					),
				},
				{
					path: "/stacks",
					add: globals(
						number("num", 666),
					),
				},
				{
					path: "/stacks",
					add: exportAsLocals(
						expr("str_local", "global.str"),
						expr("name_local", "terramate.name"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack": locals(
						str("name_local", "stack"),
						number("num_local", 666),
						str("path_local", "/stacks/stack"),
						str("str_local", "string"),
					),
				},
			},
		},
		{
			name: "multiple stacks with exported locals and globals from parent dirs",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/",
					add: exportAsLocals(
						expr("num_local", "global.num"),
						expr("path_local", "terramate.path"),
					),
				},
				{
					path: "/stacks",
					add: globals(
						number("num", 666),
					),
				},
				{
					path: "/stacks",
					add: exportAsLocals(
						expr("str_local", "global.str"),
						expr("name_local", "terramate.name"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-1": locals(
						str("name_local", "stack-1"),
						number("num_local", 666),
						str("path_local", "/stacks/stack-1"),
						str("str_local", "string"),
					),
					"/stacks/stack-2": locals(
						str("name_local", "stack-2"),
						number("num_local", 666),
						str("path_local", "/stacks/stack-2"),
						str("str_local", "string"),
					),
				},
			},
		},
		{
			name: "multiple stacks with specific exported locals and globals from parent dirs",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/stacks",
					add: globals(
						number("num", 666),
					),
				},
				{
					path: "/stacks/stack-1",
					add: exportAsLocals(
						expr("str_local", "global.str"),
						expr("name_local", "terramate.name"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: exportAsLocals(
						expr("num_local", "global.num"),
						expr("path_local", "terramate.path"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-1": locals(
						str("name_local", "stack-1"),
						str("str_local", "string"),
					),
					"/stacks/stack-2": locals(
						number("num_local", 666),
						str("path_local", "/stacks/stack-2"),
					),
				},
			},
		},
		{
			name: "multiple stacks selecting single stack with working dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-1",
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/stacks/stack-1",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-1": locals(
						str("str_local", "string"),
					),
				},
			},
		},
		{
			name: "stacks getting code generation config from parent",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/stacks",
					add: terramate(
						cfg(
							gen(
								str("locals_filename", "locals.tf"),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-1": locals(
						str("str_local", "string"),
					),
					"/stacks/stack-2": locals(
						str("str_local", "string"),
					),
				},
			},
		},
		{
			name: "stacks with code gen cfg filtered by working dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-2",
			configs: []hclblock{
				{
					path: "/",
					add: globals(
						str("str", "string"),
					),
				},
				{
					path: "/stacks",
					add: terramate(
						cfg(
							gen(
								str("locals_filename", "locals.tf"),
							),
						),
					),
				},
				{
					path: "/stacks/stack-1",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
				{
					path: "/stacks/stack-2",
					add: exportAsLocals(
						expr("str_local", "global.str"),
					),
				},
			},
			want: want{
				stacksLocals: map[string]*hclwrite.Block{
					"/stacks/stack-2": locals(
						str("str_local", "string"),
					),
				},
			},
		},
		{
			name: "working dir has no stacks inside",
			layout: []string{
				"s:stack",
				"d:somedir",
			},
			workingDir: "somedir",
			configs: []hclblock{
				{
					path: "/stack",
					add: exportAsLocals(
						expr("path", "terramate.path"),
					),
				},
			},
			want: want{},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.DefaultFilename, cfg.add.String())
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			report := generate.Do(s.RootDir(), workingDir)
			assertReportHasError(t, report, tcase.want.err)

			for stackPath, wantHCLBlock := range tcase.want.stacksLocals {
				stackRelPath := stackPath[1:]
				want := wantHCLBlock.String()
				stack := s.StackEntry(stackRelPath)
				got := string(stack.ReadGeneratedLocals())

				assertHCLEquals(t, got, want)
			}
			// TODO(katcipis): Add proper checks for extraneous generated code.
			// For now we validated wanted files are there, but not that
			// we may have new unwanted files being generated by a bug.
		})
	}
}

func TestWontOverwriteManuallyDefinedLocals(t *testing.T) {
	const (
		manualLocals = "some manual stuff"
	)

	exportLocalsCfg := exportAsLocals(expr("a", "terramate.path"))

	s := sandbox.New(t)
	s.BuildTree([]string{
		fmt.Sprintf("f:%s:%s", config.DefaultFilename, exportLocalsCfg.String()),
		"s:stack",
		fmt.Sprintf("f:stack/%s:%s", generate.LocalsFilename, manualLocals),
	})

	report := generate.Do(s.RootDir(), s.RootDir())
	assertReportHasError(t, report, generate.ErrManualCodeExists)

	stack := s.StackEntry("stack")
	locals := string(stack.ReadGeneratedLocals())
	assert.EqualStrings(t, manualLocals, locals, "locals altered by generate")
}

func TestExportedLocalsOverwriting(t *testing.T) {
	firstConfig := exportAsLocals(expr("a", "terramate.path"))
	firstWant := locals(str("a", "/stack"))

	s := sandbox.New(t)
	stack := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(firstConfig.String())

	assertReportHasNoError(t, generate.Do(s.RootDir(), s.RootDir()))

	got := string(stack.ReadGeneratedLocals())
	assertHCLEquals(t, got, firstWant.String())

	secondConfig := exportAsLocals(expr("b", "terramate.name"))
	secondWant := locals(str("b", "stack"))
	rootConfig.Write(secondConfig.String())

	assertReportHasNoError(t, generate.Do(s.RootDir(), s.RootDir()))

	got = string(stack.ReadGeneratedLocals())
	assertHCLEquals(t, got, secondWant.String())
}
