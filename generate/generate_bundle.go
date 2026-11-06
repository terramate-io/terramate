// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
)

func (g *gState) generateBundleStacks(report *genreport.Report, allowCreate bool) {
	// Creating a stack reloads its subtree, which replaces the tree nodes of all stacks nested
	// below it. Therefore all stacks are created first, before anything is attached to their nodes.
	createdBy := map[project.Path]*config.Bundle{}
	for _, bundle := range g.registry.Bundles {
		for _, stack := range bundle.Stacks {
			if g.createBundleStack(bundle, stack, report, allowCreate) {
				createdBy[stack.Dir] = bundle
			}
		}
	}

	for _, bundle := range g.registry.Bundles {
		for _, stack := range bundle.Stacks {
			g.setupBundleStack(bundle, stack, createdBy[stack.Dir] == bundle)
		}
	}
}

// createBundleStack creates the stack if it doesn't exist yet and reports if it was created.
func (g *gState) createBundleStack(bundle *config.Bundle, stackMeta config.StackMetadata, report *genreport.Report, allowCreate bool) bool {
	logger := log.With().
		Str("action", "generate.createBundleStack()").
		Str("bundle", bundle.Name).
		Str("stack", stackMeta.Name).
		Logger()

	if stackMeta.Skipped {
		logger.Debug().Msg("skipping stack because of condition attribute")
		return false
	}

	stackTree, ok := g.root.Lookup(stackMeta.Dir)
	if ok && stackTree.IsStack() {
		logger.Debug().Msg("stack already exists: skipping")
		return false
	}

	if !allowCreate {
		report.AddFailure(stackMeta.Dir, errors.E("stack not generated"))
		return false
	}

	// We only write ID. Other attributes will be inherited dynamically from the bundle stack.
	stackCfg := config.Stack{
		Dir: stackMeta.Dir,
		ID:  uuid.NewString(),
	}

	logger.Debug().Msg("creating stack")

	err := stack.Create(g.root, stackCfg)
	if err != nil {
		report.AddFailure(stackMeta.Dir, err)
		return false
	}

	logger.Debug().Msg("loading stack")

	err = g.root.LoadSubTree(stackMeta.Dir)
	if err != nil {
		report.AddFailure(stackMeta.Dir, err)
		return false
	}

	logger.Debug().Msg("stack loaded successfully")

	if _, ok := g.root.Lookup(stackMeta.Dir); !ok {
		panic(errors.E(errors.ErrInternal, "just created stack %s cannot be loaded", stackMeta.Dir))
	}

	logger.Debug().Msg("adding created file to report")

	dirReport := genreport.Dir{}
	dirReport.AddCreatedFile(stack.DefaultFilename)
	report.AddDirReport(stackMeta.Dir, dirReport)
	return true
}

// setupBundleStack attaches the bundle's runtime components to the stack.
// If the stack was just created by this bundle, its metadata is set as well.
func (g *gState) setupBundleStack(bundle *config.Bundle, stackMeta config.StackMetadata, created bool) {
	if stackMeta.Skipped {
		return
	}

	stackTree, ok := g.root.Lookup(stackMeta.Dir)
	if !ok || !stackTree.IsStack() {
		// The stack doesn't exist because creating it failed or wasn't allowed.
		return
	}

	if created {
		st := stackTree.Node.Stack

		st.Name = stackMeta.Name
		st.Description = stackMeta.Description
		st.Tags = stackMeta.Tags
		st.After = stackMeta.After
		st.Before = stackMeta.Before
		st.Wants = stackMeta.Wants
		st.WantedBy = stackMeta.WantedBy
		st.Watch = stackMeta.Watch
	}

	// attaching its runtime components
	stackTree.Node.Components = mergeComponentList(stackTree.Node.Components, stackMeta.Components, bundle.Source)
}

func mergeComponentList(dst, src []*hcl.Component, bundleSource string) []*hcl.Component {
	for _, srcComp := range src {
		contains := false
		for _, dstComp := range dst {
			if srcComp.Name == dstComp.Name {
				contains = true
				break
			}
		}
		if contains {
			continue
		}
		// Clone
		compCopy := *srcComp
		compCopy.FromBundleSource = bundleSource
		dst = append(dst, &compCopy)
	}
	return dst
}
