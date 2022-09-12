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

package e2etest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestVendorModule(t *testing.T) {
	const (
		ref      = "main"
		filename = "test.txt"
		content  = "test"
	)

	repoSandbox := sandbox.New(t)
	repoSandbox.RootEntry().CreateFile(filename, content)

	repoGit := repoSandbox.Git()
	repoGit.CommitAll("add file")

	gitSource := "git::file://" + repoSandbox.RootDir()

	checkVendoredFiles := func(t *testing.T, res runResult, vendordir string) {
		t.Helper()

		assertRunResult(t, res, runExpected{IgnoreStdout: true})

		clonedir := filepath.Join(vendordir, repoSandbox.RootDir(), "main")

		got := test.ReadFile(t, clonedir, filename)
		assert.EqualStrings(t, content, string(got))
	}

	// Check default config and then different configuration precedences
	s := sandbox.New(t)

	t.Run("default configuration", func(t *testing.T) {
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, filepath.Join(s.RootDir(), "modules"))
	})

	t.Run("root configuration", func(t *testing.T) {
		rootcfg := "/from/root/cfg"
		s.RootEntry().CreateFile("vendor.tm", vendorHCLConfig(rootcfg))
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, filepath.Join(s.RootDir(), rootcfg))
	})

	t.Run(".terramate configuration", func(t *testing.T) {
		dotTerramateCfg := "/from/dottm/cfg"
		dotTerramateDir := s.RootEntry().CreateDir(".terramate")

		dotTerramateDir.CreateFile("vendor.tm", vendorHCLConfig(dotTerramateCfg))
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, filepath.Join(s.RootDir(), dotTerramateCfg))
	})

	t.Run("CLI configuration", func(t *testing.T) {
		cliCfg := "/from/cli/cfg"
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", "--dir", cliCfg, gitSource, "main")
		checkVendoredFiles(t, res, filepath.Join(s.RootDir(), cliCfg))
	})

	t.Run("CLI configuration with subdir", func(t *testing.T) {
		cliCfg := "/with/subdir"
		gitSource := gitSource + "//subdir"
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", "--dir", cliCfg, gitSource, "main")
		checkVendoredFiles(t, res, filepath.Join(s.RootDir(), cliCfg))
	})
}

func TestVendorModuleRecursive1DependencyIsPatched(t *testing.T) {
	const moduleFileTemplate = `module "test" { source = "%s" }`
	depsSandbox := sandbox.New(t)
	depsSandbox.RootEntry().CreateFile("main.tf", ``)

	repoGit := depsSandbox.Git()
	repoGit.CommitAll("add file")

	depsGitSource := "git::file://" + depsSandbox.RootDir() + "?ref=main"

	moduleSandbox := sandbox.New(t)

	moduleSandbox.RootEntry().CreateFile("main.tf",
		fmt.Sprintf(moduleFileTemplate, depsGitSource))

	repoGit = moduleSandbox.Git()
	repoGit.CommitAll("add file")

	gitSource := "git::file://" + moduleSandbox.RootDir()

	s := sandbox.New(t)

	tmcli := newCLI(t, s.RootDir())
	res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
	assertRunResult(t, res, runExpected{IgnoreStdout: true})

	vendordir := filepath.Join(s.RootDir(), "modules")
	moduleDir := filepath.Join(vendordir, moduleSandbox.RootDir(), "main")
	depsDir := filepath.Join(vendordir, depsSandbox.RootDir(), "main")

	got := test.ReadFile(t, moduleDir, "main.tf")
	assert.EqualStrings(t,
		fmt.Sprintf(moduleFileTemplate, "../../../001/sandbox/main"),
		string(got))

	got = test.ReadFile(t, depsDir, "main.tf")
	assert.EqualStrings(t, "", string(got))
}

func vendorHCLConfig(dir string) string {
	return fmt.Sprintf(`
		vendor {
		  dir = %q
		}
	`, dir)
}
