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
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
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

func TestModVendorRecursiveMustPatchAlreadyVendoredModules(t *testing.T) {
	// This reproduces the bug below:
	//   - ModA depends on ModZ
	//   - ModB depends on ModZ
	// executing:
	//   - tm vendor download ModA <ref>
	//   - tm vendor download ModB <ref>
	// will patch the ModA files to use ../modules/ModZ/...
	// but will not for ModB because ModZ is already vendored.

	const filename = "main.tf"

	setupModuleGit := func(name string, ref string, deps ...string) string {
		moduleRepo := sandbox.New(t)
		var mainContent string
		for _, dep := range deps {
			if mainContent != "" {
				mainContent += "\n"
			}

			mainContent += Module(
				Labels(name),
				Str("source", dep+"?ref="+ref),
			).String()
		}

		moduleRepo.RootEntry().CreateFile(filename, mainContent)
		repoGit := moduleRepo.Git()
		repoGit.CommitAll("add file")
		repoGit.CheckoutNew("test")
		repoGit.Push("main")
		repoGit.Push("test")
		return "git::file://" + moduleRepo.RootDir()
	}

	modZ := setupModuleGit("modZ", "main")
	modA := setupModuleGit("modA", "main", modZ)
	modB := setupModuleGit("modB", "main", modZ)

	// modC is used to test of a different ref in the dependency must also vendor
	// and patch.
	modC := setupModuleGit("modC", "test", modZ)

	// setup project
	s := sandbox.NoGit(t)
	s.RootEntry().CreateConfig(Terramate(
		Config(),
	).String())
	tmcli := newCLI(t, s.RootDir())
	res := tmcli.run("experimental", "vendor", "download", modA, "main")
	assertRunResult(t, res, runExpected{IgnoreStdout: true})
	res = tmcli.run("experimental", "vendor", "download", modB, "main")
	assertRunResult(t, res, runExpected{IgnoreStdout: true})
	res = tmcli.run("experimental", "vendor", "download", modC, "main")
	assertRunResult(t, res, runExpected{IgnoreStdout: true})

	modsrcA, err := tf.ParseSource(modA + "?ref=main")
	assert.NoError(t, err)
	modsrcB, err := tf.ParseSource(modB + "?ref=main")
	assert.NoError(t, err)
	modsrcC, err := tf.ParseSource(modC + "?ref=main")
	assert.NoError(t, err)
	modsrcZmain, err := tf.ParseSource(modZ + "?ref=main")
	assert.NoError(t, err)
	modsrcZtest, err := tf.ParseSource(modZ + "?ref=test")
	assert.NoError(t, err)

	modFileA := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), "modules", modsrcA),
		filename,
	)

	modFileB := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), "modules", modsrcB),
		filename,
	)

	modFileC := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), "modules", modsrcC),
		filename,
	)

	wantedFileContent := func(name string, modsrc, modsrcDep tf.Source) string {
		relPath, err := filepath.Rel(
			modvendor.AbsVendorDir(s.RootDir(), "modules", modsrc),
			modvendor.AbsVendorDir(s.RootDir(), "modules", modsrcDep))
		assert.NoError(t, err)
		return Module(
			Labels(name),
			Str("source", relPath),
		).String()
	}

	test.AssertFileContentEquals(t, modFileA, wantedFileContent("modA", modsrcA, modsrcZmain))
	test.AssertFileContentEquals(t, modFileB, wantedFileContent("modB", modsrcB, modsrcZmain))
	test.AssertFileContentEquals(t, modFileC, wantedFileContent("modC", modsrcC, modsrcZtest))
}

func vendorHCLConfig(dir string) string {
	return fmt.Sprintf(`
		vendor {
		  dir = %q
		}
	`, dir)
}
