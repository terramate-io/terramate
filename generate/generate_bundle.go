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
	"github.com/terramate-io/terramate/stack"
)

func (g *gState) generateBundleStacks(report *genreport.Report, allowCreate bool) {
	for _, bundle := range g.registry.Bundles {
		for _, stack := range bundle.Stacks {
			g.generateBundleStack(bundle, stack, report, allowCreate)
		}
	}
}

func (g *gState) generateBundleStack(bundle *config.Bundle, stackMeta config.StackMetadata, report *genreport.Report, allowCreate bool) {
	logger := log.With().
		Str("action", "generate.generateBundleStack()").
		Str("bundle", bundle.Name).
		Str("stack", stackMeta.Name).
		Logger()

	if stackMeta.Skipped {
		logger.Debug().Msg("skipping stack because of condition attribute")
		return
	}

	stackTree, ok := g.root.Lookup(stackMeta.Dir)
	if ok && stackTree.IsStack() {
		logger.Debug().Msg("stack already exists: skipping")
		// attaching its runtime components
		stackTree.Node.Components = mergeComponentList(stackTree.Node.Components, stackMeta.Components, bundle.Source)
		return
	}

	if !allowCreate {
		report.AddFailure(stackMeta.Dir, errors.E("stack not generated"))
		return
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
		return
	}

	logger.Debug().Msg("loading stack")

	err = g.root.LoadSubTree(stackMeta.Dir)
	if err != nil {
		report.AddFailure(stackMeta.Dir, err)
		return
	}

	logger.Debug().Msg("stack loaded successfully")

	stackTree, ok = g.root.Lookup(stackMeta.Dir)
	if !ok {
		panic(errors.E(errors.ErrInternal, "just created stack %s cannot be loaded", stackMeta.Dir))
	}

	st := stackTree.Node.Stack

	st.Name = stackMeta.Name
	st.Description = stackMeta.Description
	st.Tags = stackMeta.Tags
	st.After = stackMeta.After
	st.Before = stackMeta.Before
	st.Wants = stackMeta.Wants
	st.WantedBy = stackMeta.WantedBy
	st.Watch = stackMeta.Watch

	logger.Debug().Msg("adding created file to report")

	dirReport := genreport.Dir{}
	dirReport.AddCreatedFile(stack.DefaultFilename)
	report.AddDirReport(stackMeta.Dir, dirReport)

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
