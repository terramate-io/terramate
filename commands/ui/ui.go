// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"context"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/scaffold/manifest"
	"github.com/terramate-io/terramate/stdlib"
)

// Spec is the command specification for the prompt command.
type Spec struct {
	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "prompt" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the prompt command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	resolveAPI, err := di.Get[resolve.API](ctx)
	if err != nil {
		return err
	}

	root := s.engine.Config()
	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())
	setupRootGlobals(root, evalctx)

	reg, err := engine.EvalProjectBundles(root, resolveAPI, evalctx, true)
	if err != nil {
		return err
	}

	manifestSources, err := lookupPackageSources(s.engine.RootNode(), evalctx)
	if err != nil {
		return err
	}

	localBundleDefs, err := config.ListLocalBundleDefinitions(root, evalctx, project.NewPath("/bundles"))
	if err != nil {
		return err
	}

	var collections []*manifest.Collection
	if len(localBundleDefs) > 0 {
		collections = append(collections, makeLocalCollection(localBundleDefs))
	}
	for _, manifestSrc := range manifestSources {
		c, err := s.loadManifest(manifestSrc, resolveAPI)
		if err != nil {
			return err
		}
		collections = append(collections, c...)
	}

	est := &EngineState{
		Context:         ctx,
		CLI:             cli,
		WorkingDir:      s.workingDir,
		Root:            root,
		Evalctx:         evalctx,
		ResolveAPI:      resolveAPI,
		Registry:        &Registry{Registry: reg},
		LocalBundleDefs: localBundleDefs,
		Collections:     collections,
		CLIConfig:       cli.Config(),
	}

	m := NewModel(est)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return errors.E(err, "failed to start ui")
	}

	model := finalModel.(Model)
	if model.err != nil {
		return model.err
	}

	if model.cancelled {
		printChangeLog(s.printers, model.changeLog)
		return nil
	}

	printChangeLog(s.printers, model.changeLog)
	return nil
}

func (s *Spec) loadManifest(manifestSrc string, resolveAPI resolve.API) ([]*manifest.Collection, error) {
	cfg := s.engine.Config()
	rootdir := cfg.HostDir()

	var manifestPath string
	isLocalSource := strings.HasPrefix(manifestSrc, "/")

	// File in the current repo.
	if isLocalSource {
		// Must specify the exact file.
		manifestPath = filepath.Join(rootdir, manifestSrc)
	} else {
		// First resolve the source to a dir, then append the default filename.
		manifestProjPath, err := resolveAPI.Resolve(rootdir, manifestSrc, resolve.Manifest, true)
		if err != nil {
			return nil, err
		}
		manifestPath = project.AbsPath(rootdir, manifestProjPath.String())
	}

	pkgs, err := manifest.LoadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	// Fall back to remote source if location is not set.
	for _, p := range pkgs {
		if p.Location == "" && !isLocalSource {
			p.Location = manifestSrc
		}
	}

	return pkgs, nil
}

func makeLocalCollection(localBundleDefs []config.BundleDefinitionEntry) *manifest.Collection {
	bundles := make([]manifest.Bundle, 0, len(localBundleDefs))
	for _, b := range localBundleDefs {
		bundles = append(bundles, manifest.Bundle{
			Name:        b.Metadata.Name,
			Class:       b.Metadata.Class,
			Version:     b.Metadata.Version,
			Description: b.Metadata.Description,
			// We omit path as it is not needed for display.
		})
	}

	return &manifest.Collection{
		Name:        "Local Repository",
		Description: "Bundles from local /bundles directory",
		Location:    "/bundles",
		Bundles:     bundles,
	}
}

// printChangeLog prints a plain-text summary of all changes made during the session.
func printChangeLog(printers printer.Printers, log []string) {
	if len(log) == 0 {
		return
	}
	printers.Stdout.Println("")
	printers.Stdout.Println("Changes made during this session:")
	printers.Stdout.Println("")
	for _, line := range log {
		printers.Stdout.Println("  " + line)
	}
	printers.Stdout.Println("")
}

func setupRootGlobals(root *config.Root, evalctx *eval.Context) {
	// Add globals from root to the context.
	// This is a best effort, there might be undefined stack. variables, so we ignore any errors.
	// Expressions that are evaluatable will still be set.
	_ = globals.ForDir(root, project.NewPath("/"), evalctx)
}

func lookupPackageSources(cfg hcl.Config, evalctx *eval.Context) ([]string, error) {
	if cfg.Scaffold != nil {
		scaffold, err := config.EvalScaffold(evalctx, cfg.Scaffold)
		if err != nil {
			return nil, err
		}
		return scaffold.PackageSources, nil
	}
	return nil, nil
}
