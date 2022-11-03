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
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/test"
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/mineiros-io/terramate/tf"
	"go.lsp.dev/uri"
)

func TestVendorModule(t *testing.T) {
	t.Parallel()

	const (
		ref      = "main"
		filename = "test.txt"
		content  = "test"
	)

	repoSandbox := sandbox.New(t)
	repoSandbox.RootEntry().CreateFile(filename, content)

	repoGit := repoSandbox.Git()
	repoGit.CommitAll("add file")

	gitSource := newLocalSource(repoSandbox.RootDir())
	modsrc, err := tf.ParseSource(gitSource + "?ref=main")
	assert.NoError(t, err)

	s := sandbox.New(t)

	// Check default config and then different configuration precedences
	checkVendoredFiles := func(t *testing.T, res runResult, vendordir project.Path) {
		t.Helper()

		assertRunResult(t, res, runExpected{IgnoreStdout: true})

		clonedir := modvendor.AbsVendorDir(s.RootDir(), vendordir, modsrc)

		got := test.ReadFile(t, clonedir, filename)
		assert.EqualStrings(t, content, string(got))
	}

	t.Run("default configuration", func(t *testing.T) {
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, project.NewPath("/modules"))
	})

	t.Run("root configuration", func(t *testing.T) {
		rootcfg := "/from/root/cfg"
		s.RootEntry().CreateFile("vendor.tm", vendorHCLConfig(rootcfg))
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, project.NewPath(rootcfg))
	})

	t.Run(".terramate configuration", func(t *testing.T) {
		dotTerramateCfg := "/from/dottm/cfg"
		dotTerramateDir := s.RootEntry().CreateDir(".terramate")

		dotTerramateDir.CreateFile("vendor.tm", vendorHCLConfig(dotTerramateCfg))
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", gitSource, "main")
		checkVendoredFiles(t, res, project.NewPath(dotTerramateCfg))
	})

	t.Run("CLI configuration", func(t *testing.T) {
		cliCfg := "/from/cli/cfg"
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", "--dir", cliCfg, gitSource, "main")
		checkVendoredFiles(t, res, project.NewPath(cliCfg))
	})

	t.Run("CLI configuration with subdir", func(t *testing.T) {
		cliCfg := "/with/subdir"
		gitSource := gitSource + "//subdir"
		tmcli := newCLI(t, s.RootDir())
		res := tmcli.run("experimental", "vendor", "download", "--dir", cliCfg, gitSource, "main")
		checkVendoredFiles(t, res, project.NewPath(cliCfg))
	})
}

func TestVendorModuleRecursiveDependencyIsPatched(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t)
	s.BuildTree([]string{
		"g:repos/target",
		"g:repos/dep",
		"g:tmroot",
	})

	depDir := s.DirEntry("repos/dep")
	depDir.CreateFile("main.tf", ``)
	depGit := depDir.Git()
	depGit.CommitAll("add file")
	depGit.Push("main")

	depGitSource := newLocalSource(depDir.Path()) + "?ref=main"
	depmodsrc, err := tf.ParseSource(depGitSource)
	assert.NoError(t, err)

	const moduleFileTemplate = `module "test" { source = "%s" }`
	targetDir := s.DirEntry("repos/target")
	targetDir.CreateFile("main.tf",
		fmt.Sprintf(moduleFileTemplate, depGitSource))

	targetGit := targetDir.Git()
	targetGit.CommitAll("add file")
	targetGit.Push("main")

	targetGitSource := newLocalSource(targetDir.Path())
	targetmodsrc, err := tf.ParseSource(targetGitSource + "?ref=main")
	assert.NoError(t, err)

	tmrootDir := s.DirEntry("tmroot")
	tmcli := newCLI(t, tmrootDir.Path())
	res := tmcli.run("experimental", "vendor", "download", targetGitSource, "main")
	assertRunResult(t, res, runExpected{
		IgnoreStdout: true,
	})

	vendordir := project.NewPath("/modules")
	moduleDir := modvendor.AbsVendorDir(tmrootDir.Path(), vendordir, targetmodsrc)
	depsDir := modvendor.AbsVendorDir(tmrootDir.Path(), vendordir, depmodsrc)

	got := test.ReadFile(t, moduleDir, "main.tf")
	assert.EqualStrings(t,
		fmt.Sprintf(moduleFileTemplate, "../../dep/main"),
		string(got))

	got = test.ReadFile(t, depsDir, "main.tf")
	assert.EqualStrings(t, "", string(got))
}

func TestModVendorRecursiveMustPatchAlreadyVendoredModules(t *testing.T) {
	t.Parallel()

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
		return newLocalSource(moduleRepo.RootDir())
	}

	modZ := setupModuleGit("modZ", "main")
	modA := setupModuleGit("modA", "main", modZ)
	modB := setupModuleGit("modB", "main", modZ)

	// modC is used to test for a different ref in the dependency. In this case,
	// it must also vendor and patch.
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

	const vendorDir = project.Path("/modules")

	modFileA := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), vendorDir, modsrcA),
		filename,
	)

	modFileB := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), vendorDir, modsrcB),
		filename,
	)

	modFileC := filepath.Join(
		modvendor.AbsVendorDir(s.RootDir(), vendorDir, modsrcC),
		filename,
	)

	wantedFileContent := func(name string, modsrc, modsrcDep tf.Source) string {
		relPath, err := filepath.Rel(
			modvendor.AbsVendorDir(s.RootDir(), vendorDir, modsrc),
			modvendor.AbsVendorDir(s.RootDir(), vendorDir, modsrcDep))
		assert.NoError(t, err)
		return Module(
			Labels(name),
			Str("source", filepath.ToSlash(relPath)),
		).String()
	}

	test.AssertFileContentEquals(t, modFileA, wantedFileContent("modA", modsrcA, modsrcZmain))
	test.AssertFileContentEquals(t, modFileB, wantedFileContent("modB", modsrcB, modsrcZmain))
	test.AssertFileContentEquals(t, modFileC, wantedFileContent("modC", modsrcC, modsrcZtest))
}

func newLocalSource(path string) string {
	return "git::" + string(uri.File(path))
}

func vendorHCLConfig(dir string) string {
	return fmt.Sprintf(`
		vendor {
		  dir = %q
		}
	`, dir)
}
