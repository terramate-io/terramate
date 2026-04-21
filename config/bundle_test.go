// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"sort"
	"testing"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stdlib"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func defineBundleHCL(name string) string {
	return Block("define",
		Labels("bundle"),
		Block("metadata",
			Str("class", "test_class"),
			Str("name", name),
			Str("version", "1.0.0"),
		),
	).String()
}

func newEvalCtxForRoot(root *config.Root) *eval.Context {
	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())
	return evalctx
}

func sourcesOf(entries []config.BundleDefinitionEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Source)
	}
	sort.Strings(out)
	return out
}

func TestListLocalBundleDefinitionsFromRoot(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:/bundles/a/define.tm:" + defineBundleHCL("bundle_a"),
		"f:/custom/place/b/define.tm:" + defineBundleHCL("bundle_b"),
		"f:/deeply/nested/path/c/define.tm:" + defineBundleHCL("bundle_c"),
	})

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalBundleDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)

	got := sourcesOf(entries)
	want := []string{"/bundles/a", "/custom/place/b", "/deeply/nested/path/c"}
	assert.EqualInts(t, len(want), len(got), "got: %v", got)
	for i, w := range want {
		assert.EqualStrings(t, w, got[i])
	}
}

func TestListLocalBundleDefinitionsSkipsInstalledRemotePackages(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:/bundles/a/define.tm:" + defineBundleHCL("bundle_a"),
		"f:/.terramate/bundles/remote/define.tm:" + defineBundleHCL("remote_bundle"),
	})

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalBundleDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)

	got := sourcesOf(entries)
	want := []string{"/bundles/a"}
	assert.EqualInts(t, len(want), len(got), "got: %v", got)
	for i, w := range want {
		assert.EqualStrings(t, w, got[i])
	}
}

func TestListLocalBundleDefinitionsEmpty(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalBundleDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(entries))
}

/*

func TestEvalBundle(t *testing.T) {
	t.Parallel()

	evalctx := eval.NewContext(map[string]function.Function{})

	inputsBlock := &ast.MergedBlock{
		Type: "inputs",
		Attributes: map[string]ast.Attribute{
			"test": {
				Attribute: &hhcl.Attribute{
					Name: "b",
					Expr: &hclsyntax.LiteralValueExpr{
						Val: cty.StringVal("value from inputs block"),
					},
				},
			},
		},
	}

	bundle := &prohcl.Bundle{
		Name: "my_bundle",
		Source: &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: "source",
				Expr: &hclsyntax.LiteralValueExpr{
					Val: cty.StringVal("source"),
				},
			},
		},
		InputsAttr: &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: "inputs",
				Expr: &hclsyntax.ObjectConsExpr{
					Items: []hclsyntax.ObjectConsItem{
						{
							KeyExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("a"),
							},
							ValueExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("test"),
							},
						},
						{
							KeyExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("b"),
							},
							ValueExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("test"),
							},
						},
					},
				},
			},
		},
		Inputs: inputsBlock,
	}

	evaluated, err := config.EvalBundle(evalctx, bundle)
	if err != nil {
		t.Fatalf("failed to evaluate bundle: %s", err)
	}

	expected := config.Bundle{
		Name:   "my_bundle",
		Source: "source",
		Inputs: map[string]cty.Value{
			"a": cty.StringVal("test"),
			"b": cty.StringVal("value from inputs block"),
		},
	}
	if diff := cmp.Diff(expected, evaluated, cmpopts.IgnoreFields(config.Bundle{}, "Info", "Inputs")); diff != "" {
		t.Fatalf("expected %v, got %v", expected, evaluated)
	}
}
*/
