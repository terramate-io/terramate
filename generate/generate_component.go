// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"sort"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/generate/genfile"
	"github.com/terramate-io/terramate/generate/genhcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
)

// loadComponentGenFiles loads and evaluates all generate blocks from a component
func (g *gState) loadComponentGenFiles(
	root *config.Root,
	st *config.Stack,
	evalctx *eval.Context,
	vendorDir project.Path,
	vendorRequests chan<- event.VendorRequest,
) ([]GenFile, error) {
	var files []GenFile

	// Get component instantiations for this stack
	cfg, _ := root.Lookup(st.Dir)

	for _, comp := range cfg.Node.Components {
		evalctx := evalctx.ChildContext()
		if comp.BundleObject != nil {
			evalctx.SetNamespaceRaw("bundle", *comp.BundleObject)
		}

		evalComp, compCfg, err := config.EvalComponent(root, g.resolveAPI, evalctx, comp, g.registry, false)
		if err != nil {
			return nil, err
		}

		if evalComp.Skipped {
			continue
		}

		compCtx := evalctx.Copy()
		compCtx.SetFunction("tm_source", config.TmSourceFunc(st.Dir.String(), evalComp.Source, comp.FromBundleSource))

		// TODO(i4k): fix genfile.Load to not inherit blocks

		// Load generate blocks from component
		genFiles, err := genfile.EvalBlocks(root, compCfg.Generate.Files, st, compCtx, vendorDir, vendorRequests, g.registry, evalComp.Environment, true)
		if err != nil {
			return nil, err
		}

		genHCLs, err := genhcl.EvalBlocks(root, compCfg.Generate.HCLs, st, compCtx, vendorDir, vendorRequests, g.registry, evalComp.Environment, "component_"+comp.Name+"_")
		if err != nil {
			return nil, err
		}

		// Add all generated files
		for _, file := range genFiles {
			files = append(files, file)
		}

		for _, hcl := range genHCLs {
			files = append(files, hcl)
		}
	}

	// Sort files by label for consistent output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Label() < files[j].Label()
	})

	return files, nil
}
