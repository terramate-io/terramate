// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package create provides the package create command.
package create

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/scaffold/manifest"
	"github.com/terramate-io/terramate/stdlib"
)

// Spec is the command specification for the package publish command.
type Spec struct {
	OutputDir string

	PackageLocation    string
	PackageName        string
	PackageDescription string

	ManifestOnly bool

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "package create" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the package publish command.
func (s *Spec) Exec(_ context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	root := s.engine.Config()

	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())

	if _, err := os.Stat(s.OutputDir); err == nil || !os.IsNotExist(err) {
		return errors.E("output directory already exists: %s", s.OutputDir)
	}

	// TODO: Could both be done in a single pass.
	localBundles, err := config.ListLocalBundleDefinitions(root, project.NewPath("/bundles"))
	if err != nil {
		return err
	}

	localComponents, err := config.ListLocalComponentDefinitions(root, project.NewPath("/components"))
	if err != nil {
		return err
	}

	if s.PackageName == "" {
		s.PackageName = s.tryDetectPackageName()
	}

	pkg := manifest.Package{
		Name:        s.PackageName,
		Location:    s.PackageLocation,
		Description: s.PackageDescription,
	}

	for _, defEntry := range localBundles {
		md, err := config.EvalMetadata(root, evalctx, defEntry.Tree, &defEntry.Define.Metadata)
		if err != nil {
			return err
		}

		pkg.Bundles = append(pkg.Bundles, manifest.Bundle{
			Path:        defEntry.Tree.Dir().String(),
			Name:        md.Name,
			Class:       md.Class,
			Version:     md.Version,
			Description: md.Description,
		})
	}

	for _, defEntry := range localComponents {
		md, err := config.EvalMetadata(root, evalctx, defEntry.Tree, &defEntry.Define.Metadata)
		if err != nil {
			return err
		}

		pkg.Components = append(pkg.Components, manifest.Component{
			Path:        defEntry.Tree.Dir().String(),
			Name:        md.Name,
			Class:       md.Class,
			Version:     md.Version,
			Description: md.Description,
		})
	}

	if len(pkg.Bundles)+len(pkg.Components) == 0 {
		return errors.E("no bundles or components found for packaging")
	}

	pkgs := []*manifest.Package{&pkg}

	manifestData, err := json.MarshalIndent(&pkgs, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.OutputDir, 0775); err != nil {
		return errors.E(err, "failed to create output directory")
	}

	commit := false

	defer func() {
		if !commit {
			if err := os.RemoveAll(s.OutputDir); err != nil {
				log.Warn().Err(err).
					Msg("failed to delete incomplete output dir after failed packaging")
			}
		} else {
			s.printers.Stdout.Println("\nPackage created successfully.")
		}
	}()

	manifestPath := filepath.Join(s.OutputDir, "terramate_packages.json")

	s.printers.Stdout.Println("Package report\n")
	s.printers.Stdout.Println("Manifest:\n")
	s.printers.Stdout.Println("- " + manifestPath)

	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return errors.E(err, "failed to write manifest file")
	}

	if s.ManifestOnly {
		commit = true
		return nil
	}

	hostdir := root.HostDir()

	filterFunc := func(_ string, entry os.DirEntry) bool {
		if entry.IsDir() {
			return true
		}
		if strings.HasSuffix(entry.Name(), ".tm.hcl") {
			s.printers.Stdout.Println("\t" + entry.Name())
			return true
		}
		return false
	}

	copyToOutputDir := func(p string) error {
		absSrcDir := project.AbsPath(hostdir, p)
		absTargetDir := filepath.Join(s.OutputDir, filepath.FromSlash(p))

		s.printers.Stdout.Println("\n- " + absTargetDir)

		if err := os.MkdirAll(absTargetDir, 0775); err != nil {
			return errors.E(err, "failed to create output directory")
		}
		if err := fs.CopyDir(absTargetDir, absSrcDir, filterFunc); err != nil {
			return errors.E(err, "failed copying to output dir from %s", p)
		}
		return nil
	}

	if len(pkg.Bundles) > 0 {
		s.printers.Stdout.Println("\nBundles:")
		for _, bundle := range pkg.Bundles {
			if err := copyToOutputDir(bundle.Path); err != nil {
				return err
			}
		}
	}
	if len(pkg.Components) > 0 {
		s.printers.Stdout.Println("\nComponents:")
		for _, component := range pkg.Components {
			if err := copyToOutputDir(component.Path); err != nil {
				return err
			}
		}
	}

	commit = true
	return nil
}

func (s *Spec) tryDetectPackageName() string {
	g, err := git.WithConfig(git.Config{
		WorkingDir: s.workingDir,
	})
	if err != nil {
		return ""
	}

	repoURL, err := g.URL("origin")
	if err != nil {
		return ""
	}

	repo, err := git.NormalizeGitURI(repoURL)
	if err != nil {
		return ""
	}
	return repo.Repo
}
