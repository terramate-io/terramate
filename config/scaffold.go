// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
)

// Scaffold is the evaluated scaffold block.
type Scaffold struct {
	PackageSources []string
}

// EvalScaffold evaluates the scaffold block.
func EvalScaffold(evalctx *eval.Context, scaffoldHCL *hcl.Scaffold) (*Scaffold, error) {
	evaluated := &Scaffold{}

	sources, err := evalStringList(evalctx, scaffoldHCL.PackageSources.Expr, "package_sources")
	if err != nil {
		return nil, err
	}

	evaluated.PackageSources = sources

	return evaluated, nil
}
