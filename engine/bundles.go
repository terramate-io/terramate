// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package engine

import (
	"context"
	"slices"

	"github.com/hashicorp/go-set/v3"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/generate/resolve"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
)

// LoadProjectBundles recursively collects all the bundles in the given dir and evaluates them.
func LoadProjectBundles(root *config.Root, resolveAPI resolve.API, evalctx *eval.Context, allowFetch bool) ([]*config.Bundle, error) {
	type entry struct {
		cfg        *config.Tree
		bundlesHCL []*hcl.BundleTemplate
	}
	var entries []entry

	for _, cfg := range root.Tree().AsList() {
		if len(cfg.Node.Bundles) == 0 {
			continue
		}
		entries = append(entries, entry{
			cfg:        cfg,
			bundlesHCL: cfg.Node.Bundles,
		})
	}
	if len(entries) == 0 {
		return nil, nil
	}

	if resolveAPI == nil {
		return nil, errors.E("internal error: ResolveAPI not initialized for bundle evaluation. please report this error.")
	}

	envs, err := config.EvalEnvironments(root, evalctx)
	if err != nil {
		return nil, err
	}

	var allBundles []*config.Bundle
	for _, e := range entries {
		// This will add the globals to the context.
		// This is a best effort, there might be undefined stack. variables, so we ignore any errors.
		// Expressions that are evaluatable will still be set.
		bundleEvalCtx := evalctx.ChildContext()

		// TODO(snk): Causes dependency cycle.
		_ = globals.ForDir(root, e.cfg.Dir(), bundleEvalCtx)

		bundles, err := config.EvalBundles(root, e.bundlesHCL, resolveAPI, bundleEvalCtx, envs, allowFetch)
		if err != nil {
			return nil, err
		}

		allBundles = append(allBundles, bundles...)
	}

	return allBundles, nil
}

func applyBundleStacks(ctx context.Context, root *config.Root) error {
	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())

	// A potential error is ignored here, because in case there are no bundles, we do not have to fail.
	// Otherwise all the tests we have need to initialize the resolve API.
	// EvalBundles will check resolveAPI later, if there are bundles to evaluate.
	resolveAPI, _ := di.Get[resolve.API](ctx)

	// TODO(snk): Cache this
	bundles, err := LoadProjectBundles(root, resolveAPI, evalctx, true)
	if err != nil {
		return err
	}

	for _, bundle := range bundles {
		for _, bundleStack := range bundle.Stacks {
			if err := applyBundleStack(root, bundleStack); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyBundleStack(root *config.Root, bundleStack config.StackMetadata) error {
	tree, ok := root.Lookup(bundleStack.Dir)
	if !ok || !tree.IsStack() {
		// Skipping, stack is not yet generated.
		return nil
	}

	// We assume the stack hasn't been loaded yet with .Stack(), so we modify only the hcl.Stack.
	st := tree.Node.Stack
	useFromBundleIfEmpty := func(stVal *string, bundleVal string) {
		if *stVal == "" {
			*stVal = bundleVal
		}
	}

	useFromBundleIfEmpty(&st.Name, bundleStack.Name)
	useFromBundleIfEmpty(&st.Description, bundleStack.Description)

	// Merge with bundle values
	st.Tags = set.From(append(st.Tags, bundleStack.Tags...)).Slice()
	slices.Sort(st.Tags)
	st.After = set.From(append(st.After, bundleStack.After...)).Slice()
	slices.Sort(st.After)
	st.Before = set.From(append(st.Before, bundleStack.Before...)).Slice()
	slices.Sort(st.Before)
	st.Wants = set.From(append(st.Wants, bundleStack.Wants...)).Slice()
	slices.Sort(st.Wants)
	st.WantedBy = set.From(append(st.WantedBy, bundleStack.WantedBy...)).Slice()
	slices.Sort(st.WantedBy)
	st.Watch = set.From(append(st.Watch, bundleStack.Watch...)).Slice()
	slices.Sort(st.Watch)

	return nil
}
