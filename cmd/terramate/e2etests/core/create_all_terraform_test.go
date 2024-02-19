// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	. "github.com/terramate-io/terramate/cmd/terramate/e2etests/internal/runner"
	"github.com/terramate-io/terramate/stack"
	errtest "github.com/terramate-io/terramate/test/errors"
	"github.com/terramate-io/terramate/test/sandbox"
)

func TestCreateWithAllTerraformModuleAtRoot(t *testing.T) {
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		`f:main.tf:terraform {
			backend "remote" {
				attr = "value"
			}
		}`,
		`f:README.md:# My module`,
	})
	tm := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		tm.Run("create", "--all-terraform"),
		RunExpected{
			Stdout: "Created stack /\n",
		},
	)
	_, err := os.Lstat(filepath.Join(s.RootDir(), stack.DefaultFilename))
	assert.NoError(t, err)
}

func TestCreateWithAllTerraformModuleDeepDownInTheTree(t *testing.T) {
	testCase := func(t *testing.T, generate bool) {
		s := sandbox.NoGit(t, true)
		const backendContent = `terraform {
		backend "remote" {
			attr = "value"
		}
	}

	`

		const providerContent = `
		provider "aws" {
			attr = 1
		}
	`

		const mixedBackendProvider = backendContent + providerContent

		s.BuildTree([]string{
			`f:prod/stacks/k8s-stack/deployment.yml:# empty file`,
			`f:prod/stacks/A/anyfile.tf:` + backendContent,
			`f:prod/stacks/A/README.md:# empty`,
			`f:prod/stacks/B/main.tf:` + providerContent,
			`f:prod/stacks/A/other-stack/main.tf:` + mixedBackendProvider,
			`f:README.md:# My module`,
			`f:generate.tm:generate_hcl "_generated.tf" {
			content {
				test = 1
			}
		}`,
		})
		tm := NewCLI(t, s.RootDir())
		args := []string{"create", "--all-terraform"}
		if !generate {
			args = append(args, "--no-generate")
		}
		AssertRunResult(t,
			tm.Run(args...),
			RunExpected{
				Stdout: `Created stack /prod/stacks/A
Created stack /prod/stacks/A/other-stack
Created stack /prod/stacks/B
`,
			},
		)

		for _, path := range []string{
			"/prod/stacks/A",
			"/prod/stacks/B",
			"/prod/stacks/A/other-stack",
		} {
			stackPath := filepath.Join(s.RootDir(), path)
			_, err := os.Lstat(filepath.Join(stackPath, stack.DefaultFilename))
			assert.NoError(t, err)

			_, err = os.Lstat(filepath.Join(stackPath, "_generated.tf"))
			if generate {
				assert.NoError(t, err)
			} else {
				errtest.Assert(t, err, os.ErrNotExist)
			}
		}
	}

	t.Run("with generation", func(t *testing.T) {
		testCase(t, true)
	})

	t.Run("without generation", func(t *testing.T) {
		testCase(t, false)
	})
}

func TestCreateWithAllTerraformSkipActualStacks(t *testing.T) {
	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		`s:stack`,
		`f:stack/main.tf:terraform {
			backend "remote" {
				attr = "value"
			}
		}`,
		`f:README.md:# My module`,
	})
	tm := NewCLI(t, s.RootDir())
	AssertRun(t, tm.Run("create", "--all-terraform"))
}

func TestCreateWithAllTerraformDetectModulesInsideStacks(t *testing.T) {
	s := sandbox.NoGit(t, true)
	const backendContent = `terraform {
		backend "remote" {
			attr = "value"
		}
	}`
	s.BuildTree([]string{
		`s:stack`,
		`f:stack/main.tf:` + backendContent,
		`f:stack/hidden/module/inside/stack/main.tf:` + backendContent,
		`f:README.md:# My module`,
	})
	tm := NewCLI(t, s.RootDir())
	AssertRunResult(t,
		tm.Run("create", "--all-terraform"),
		RunExpected{
			Stdout: "Created stack /stack/hidden/module/inside/stack\n",
		},
	)
}
