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
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestBackendConfigGeneration(t *testing.T) {
	type (
		stackcode struct {
			relpath string
			code    string
		}

		backendconfig struct {
			relpath string
			config  string
		}

		want struct {
			err    error
			stacks []stackcode
		}

		testcase struct {
			name       string
			layout     []string
			configs    []backendconfig
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
			name: "multiple stacks with no config",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"s:stacks/stack-3",
			},
		},
		{
			name:   "fails on single stack with invalid config",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend {}
}

stack{}`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name: "multiple stacks and one has invalid config fails",
			layout: []string{
				"s:stack-invalid-backend",
				"s:stack-ok-backend",
			},
			configs: []backendconfig{
				{
					relpath: "stack-invalid-backend",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend {}
}

stack{}`,
				},
				{
					relpath: "stack-ok-backend",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "valid" {}
}

stack{}`,
				},
			},
			want: want{
				err: hcl.ErrMalformedTerramateConfig,
			},
		},
		{
			name:   "single stack with config on stack and empty config",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "sometype" {}
}

stack {}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "sometype" {
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on stack and empty config label",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "" {}
}

stack {}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "" {
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on stack and config with 1 attr",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "sometype" {
    attr = "value"
  }
}

stack {}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "sometype" {
    attr = "value"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "multiple stacks with config on each stack",
			layout: []string{"s:stack-1"},
			configs: []backendconfig{
				{
					relpath: "stack-1",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "1" {
    attr = "hi"
  }
}

stack {}`,
				},
				{
					relpath: "stack-2",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "2" {
    somebool = true
  }
}

stack {}`,
				},
				{
					relpath: "stack-3",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "3" {
    somelist = ["m", "i", "n", "e", "i", "r", "o", "s"]
  }
}

stack {}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack-1",
						code: `terraform {
  backend "1" {
    attr = "hi"
  }
}
`,
					},
					{
						relpath: "stack-2",
						code: `terraform {
  backend "2" {
    somebool = true
  }
}
`,
					},
					{
						relpath: "stack-3",
						code: `terraform {
  backend "3" {
    somelist = ["m", "i", "n", "e", "i", "r", "o", "s"]
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on stack with config N attrs",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "lotsoftypes" {
    attr = "value"
    attrnumber = 5
    attrbool = true
    somelist = ["hi", "again"]
  }
}

stack {}
`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "lotsoftypes" {
    attr       = "value"
    attrbool   = true
    attrnumber = 5
    somelist   = ["hi", "again"]
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on stack with subblock",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: "stack",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "lotsoftypes" {
    attr = "value"
    block {
      attrbool   = true
      attrnumber = 5
      somelist   = ["hi", "again"]
    }
  }
}

stack {}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack",
						code: `terraform {
  backend "lotsoftypes" {
    attr = "value"
    block {
      attrbool   = true
      attrnumber = 5
      somelist   = ["hi", "again"]
    }
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config parent dir",
			layout: []string{"s:stacks/stack"},
			configs: []backendconfig{
				{
					relpath: "stacks",
					config: `terramate {
  backend "fromparent" {
    attr = "value"
  }
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack",
						code: `terraform {
  backend "fromparent" {
    attr = "value"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on basedir",
			layout: []string{"s:stacks/stack"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "basedir_config" {
    attr = 666
  }
}
`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack",
						code: `terraform {
  backend "basedir_config" {
    attr = 666
  }
}
`,
					},
				},
			},
		},
		{
			name: "multiple stacks with config on basedir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "basedir_config" {
    attr = "test"
  }
}
`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: `terraform {
  backend "basedir_config" {
    attr = "test"
  }
}
`,
					},
					{
						relpath: "stacks/stack-2",
						code: `terraform {
  backend "basedir_config" {
    attr = "test"
  }
}
`,
					},
				},
			},
		},
		{
			name: "stacks on different envs with per env config",
			layout: []string{
				"s:envs/prod/stacks/stack",
				"s:envs/staging/stacks/stack",
			},
			configs: []backendconfig{
				{
					relpath: "envs/prod",
					config: `terramate {
  backend "remote" {
    environment = "prod"
  }
}
`,
				},
				{
					relpath: "envs/staging",
					config: `terramate {
  backend "remote" {
    environment = "staging"
  }
}
`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "envs/prod/stacks/stack",
						code: `terraform {
  backend "remote" {
    environment = "prod"
  }
}
`,
					},
					{
						relpath: "envs/staging/stacks/stack",
						code: `terraform {
  backend "remote" {
    environment = "staging"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "single stack with config on stack and N attrs using metadata",
			layout: []string{"s:stack-metadata"},
			configs: []backendconfig{
				{
					relpath: "stack-metadata",
					config: `terramate {
  required_version = "~> 0.0.0"
  backend "metadata" {
    name = terramate.name
    path = terramate.path
    somelist = [terramate.name, terramate.path]
  }
}
stack {
  name = "custom-name"
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stack-metadata",
						code: `terraform {
  backend "metadata" {
    name     = "custom-name"
    path     = "/stack-metadata"
    somelist = ["custom-name", "/stack-metadata"]
  }
}
`,
					},
				},
			},
		},
		{
			name:   "multiple stacks with config on root dir using metadata",
			layout: []string{"s:stacks/stack-1", "s:stacks/stack-2"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "metadata" {
    name = terramate.name
    path = terramate.path
    interpolation = "interpolate-${terramate.name}-fun-${terramate.path}"
  }
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: `terraform {
  backend "metadata" {
    interpolation = "interpolate-stack-1-fun-/stacks/stack-1"
    name          = "stack-1"
    path          = "/stacks/stack-1"
  }
}
`,
					},
					{
						relpath: "stacks/stack-2",
						code: `terraform {
  backend "metadata" {
    interpolation = "interpolate-stack-2-fun-/stacks/stack-2"
    name          = "stack-2"
    path          = "/stacks/stack-2"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "multiple stacks with config on root dir using metadata and tf functions",
			layout: []string{"s:stacks/stack-1", "s:stacks/stack-2"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "metadata" {
    funcfun  = replace(terramate.path, "/","-")
    funcfunb = "testing-funcs-${replace(terramate.path, "/",".")}"
    name     = terramate.name
    path     = terramate.path
  }
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: `terraform {
  backend "metadata" {
    funcfun  = "-stacks-stack-1"
    funcfunb = "testing-funcs-.stacks.stack-1"
    name     = "stack-1"
    path     = "/stacks/stack-1"
  }
}
`,
					},
					{
						relpath: "stacks/stack-2",
						code: `terraform {
  backend "metadata" {
    funcfun  = "-stacks-stack-2"
    funcfunb = "testing-funcs-.stacks.stack-2"
    name     = "stack-2"
    path     = "/stacks/stack-2"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "multiple stacks with config on parent dir using globals from root",
			layout: []string{"s:stacks/stack-1", "s:stacks/stack-2"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `
globals {
  bucket = "project-wide-bucket"
}`,
				},
				{
					relpath: "stacks",
					config: `terramate {
  backend "gcs" {
    bucket = global.bucket
    prefix = terramate.path
  }
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: `terraform {
  backend "gcs" {
    bucket = "project-wide-bucket"
    prefix = "/stacks/stack-1"
  }
}
`,
					},
					{
						relpath: "stacks/stack-2",
						code: `terraform {
  backend "gcs" {
    bucket = "project-wide-bucket"
    prefix = "/stacks/stack-2"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "stack with global on parent dir using config from root",
			layout: []string{"s:stacks/stack"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "gcs" {
    bucket = global.bucket
    prefix = terramate.path
  }
}`,
				},
				{
					relpath: "stacks",
					config: `
globals {
  bucket = "project-wide-bucket"
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack",
						code: `terraform {
  backend "gcs" {
    bucket = "project-wide-bucket"
    prefix = "/stacks/stack"
  }
}
`,
					},
				},
			},
		},
		{
			name: "stack overriding parent global",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "gcs" {
    bucket = global.bucket
    prefix = terramate.path
  }
}`,
				},
				{
					relpath: "stacks",
					config: `
globals {
  bucket = "project-wide-bucket"
}`,
				},
				{
					relpath: "stacks/stack-1",
					config: `
terramate {
  required_version = "~> 0.0.0"
}

stack {}

globals {
  bucket = "stack-specific-bucket"
}`,
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: `terraform {
  backend "gcs" {
    bucket = "stack-specific-bucket"
    prefix = "/stacks/stack-1"
  }
}
`,
					},
					{
						relpath: "stacks/stack-2",
						code: `terraform {
  backend "gcs" {
    bucket = "project-wide-bucket"
    prefix = "/stacks/stack-2"
  }
}
`,
					},
				},
			},
		},
		{
			name:   "reference to undefined global fails",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "gcs" {
    bucket = global.bucket
  }
}`,
				},
			},
			want: want{
				err: generate.ErrBackendConfigGen,
			},
		},
		{
			name:   "invalid global definition fails",
			layout: []string{"s:stack"},
			configs: []backendconfig{
				{
					relpath: ".",
					config: `terramate {
  backend "gcs" {
    bucket = "all good"
  }
}

globals {
  undefined_reference = global.undefined
}
`,
				},
			},
			want: want{
				err: generate.ErrLoadingGlobals,
			},
		},
		{
			name: "multiple stacks selecting single stack with working dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-1",
			configs: []backendconfig{
				{
					relpath: ".",
					config: hcldoc(
						terramate(
							backend(
								labels("gcs"),
								expr("prefix", "terramate.path"),
							),
						),
					).String(),
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: hcldoc(
							terraform(
								backend(
									labels("gcs"),
									str("prefix", "/stacks/stack-1"),
								),
							),
						).String(),
					},
				},
			},
		},
		{
			name: "stacks using parent generated code filenames",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			configs: []backendconfig{
				{
					relpath: "/stacks",
					config: hcldoc(

						terramate(
							backend(
								labels("gcs"),
								expr("prefix", "terramate.path"),
							),
							cfg(
								gen(
									str("backend_config_filename", "backend.tf"),
								),
							),
						),
					).String(),
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: hcldoc(
							terraform(
								backend(
									labels("gcs"),
									str("prefix", "/stacks/stack-1"),
								),
							),
						).String(),
					},
					{
						relpath: "stacks/stack-2",
						code: hcldoc(
							terraform(
								backend(
									labels("gcs"),
									str("prefix", "/stacks/stack-2"),
								),
							),
						).String(),
					},
				},
			},
		},
		{
			name: "stacks using parent generated code filenames filtered by working dir",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
			},
			workingDir: "stacks/stack-1",
			configs: []backendconfig{
				{
					relpath: "/stacks",
					config: hcldoc(

						terramate(
							backend(
								labels("gcs"),
								expr("prefix", "terramate.path"),
							),
							cfg(
								gen(
									str("backend_config_filename", "backend.tf"),
								),
							),
						),
					).String(),
				},
			},
			want: want{
				stacks: []stackcode{
					{
						relpath: "stacks/stack-1",
						code: hcldoc(
							terraform(
								backend(
									labels("gcs"),
									str("prefix", "/stacks/stack-1"),
								),
							),
						).String(),
					},
				},
			},
		},
		{
			name: "working dir has no stacks inside",
			layout: []string{
				"s:stacks/stack-1",
				"s:stacks/stack-2",
				"d:notstack",
			},
			workingDir: "notstack",
			configs: []backendconfig{
				{
					relpath: ".",
					config: hcldoc(
						terramate(
							backend(
								labels("gcs"),
								expr("prefix", "terramate.path"),
							),
						),
					).String(),
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
				dir := filepath.Join(s.RootDir(), cfg.relpath)
				test.WriteFile(t, dir, config.DefaultFilename, cfg.config)
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			report := generate.Do(s.RootDir(), workingDir)
			assertReportHasError(t, report, tcase.want.err)

			for _, want := range tcase.want.stacks {
				stack := s.StackEntry(want.relpath)
				got := string(stack.ReadGeneratedBackendCfg())

				assertHCLEquals(t, got, want.code)
			}
			// TODO(katcipis): Add proper checks for extraneous generated code.
			// For now we validated wanted files are there, but not that
			// we may have new unwanted files being generated by a bug.
		})
	}
}

func TestWontOverwriteManuallyDefinedBackendConfig(t *testing.T) {
	const (
		manualContents = "some manual backend configs"
	)

	backend := func(label string) *hclwrite.Block {
		b := hclwrite.BuildBlock("backend")
		b.AddLabel(label)
		return b
	}
	rootTerramateConfig := terramate(backend("test"))

	s := sandbox.New(t)
	s.BuildTree([]string{
		fmt.Sprintf("f:%s:%s", config.DefaultFilename, rootTerramateConfig.String()),
		"s:stack",
		fmt.Sprintf("f:stack/%s:%s", generate.BackendCfgFilename, manualContents),
	})

	report := generate.Do(s.RootDir(), s.RootDir())
	assertReportHasError(t, report, generate.ErrManualCodeExists)

	stack := s.StackEntry("stack")

	backendConfig := string(stack.ReadGeneratedBackendCfg())
	assert.EqualStrings(t, manualContents, backendConfig, "backend config altered by generate")
}

func TestBackendConfigOverwriting(t *testing.T) {
	backend := func(label string) *hclwrite.Block {
		b := hclwrite.BuildBlock("backend")
		b.AddLabel(label)
		return b
	}
	firstConfig := terramate(backend("first"))
	firstWant := terraform(backend("first"))

	s := sandbox.New(t)
	stack := s.CreateStack("stack")
	rootEntry := s.DirEntry(".")
	rootConfig := rootEntry.CreateConfig(firstConfig.String())

	assertReportHasNoError(t, generate.Do(s.RootDir(), s.RootDir()))

	got := string(stack.ReadGeneratedBackendCfg())
	assertHCLEquals(t, got, firstWant.String())

	secondConfig := terramate(backend("second"))
	secondWant := terraform(backend("second"))
	rootConfig.Write(secondConfig.String())

	assertReportHasNoError(t, generate.Do(s.RootDir(), s.RootDir()))

	got = string(stack.ReadGeneratedBackendCfg())
	assertHCLEquals(t, got, secondWant.String())
}
