// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"sort"
	"testing"

	"github.com/madlambda/spells/assert"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/project"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
)

func defineComponentHCL(name string) string {
	return Block("define",
		Labels("component"),
		Block("metadata",
			Str("class", "test_class"),
			Str("name", name),
			Str("version", "1.0.0"),
		),
	).String()
}

func componentDirsOf(entries []config.ComponentDefinitionEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Tree.Dir().String())
	}
	sort.Strings(out)
	return out
}

func TestListLocalComponentDefinitionsFromRoot(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:/components/a/define.tm:" + defineComponentHCL("component_a"),
		"f:/custom/place/b/define.tm:" + defineComponentHCL("component_b"),
		"f:/deeply/nested/path/c/define.tm:" + defineComponentHCL("component_c"),
	})

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalComponentDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)

	got := componentDirsOf(entries)
	want := []string{"/components/a", "/custom/place/b", "/deeply/nested/path/c"}
	assert.EqualInts(t, len(want), len(got), "got: %v", got)
	for i, w := range want {
		assert.EqualStrings(t, w, got[i])
	}
}

func TestListLocalComponentDefinitionsSkipsInstalledRemotePackages(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)
	s.BuildTree([]string{
		"f:/components/a/define.tm:" + defineComponentHCL("component_a"),
		"f:/.terramate/components/remote/define.tm:" + defineComponentHCL("remote_component"),
	})

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalComponentDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)

	got := componentDirsOf(entries)
	want := []string{"/components/a"}
	assert.EqualInts(t, len(want), len(got), "got: %v", got)
	for i, w := range want {
		assert.EqualStrings(t, w, got[i])
	}
}

func TestListLocalComponentDefinitionsEmpty(t *testing.T) {
	t.Parallel()

	s := sandbox.NoGit(t, true)

	root, err := config.LoadRoot(s.RootDir(), false)
	assert.NoError(t, err)

	entries, err := config.ListLocalComponentDefinitions(root, newEvalCtxForRoot(root), project.NewPath("/"))
	assert.NoError(t, err)
	assert.EqualInts(t, 0, len(entries))
}

/*
import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	hclsyntax "github.com/terramate-io/hcl/v2/hclsyntax"

	hhcl "github.com/terramate-io/hcl/v2"

	"github.com/terramate-io/terramate-catalyst/config"
	prohcl "github.com/terramate-io/terramate-catalyst/hcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func TestEvalComponent(t *testing.T) {
	t.Parallel()
	rootdir := t.TempDir()
	evalctx := eval.NewContext(map[string]function.Function{})

	inputsBlock := ast.NewMergedBlock("inputs", []string{})
	inputsBlock.Attributes = ast.Attributes{
		"test": {
			Attribute: &hhcl.Attribute{
				Name: "test",
				Expr: &hclsyntax.LiteralValueExpr{
					Val: cty.StringVal("value from inputs block"),
				},
			},
		},
	}

	component := prohcl.Component{
		Name: "test",
		Source: &ast.Attribute{
			Attribute: &hhcl.Attribute{
				Name: "source",
				Expr: &hclsyntax.LiteralValueExpr{
					Val: cty.StringVal("test"),
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
								Val: cty.StringVal("test"),
							},
							ValueExpr: &hclsyntax.LiteralValueExpr{
								Val: cty.StringVal("test"),
							},
						},
					},
				},
				Range: hhcl.Range{
					Filename: filepath.Join(rootdir, "test.hcl"),
					Start: hhcl.Pos{
						Line:   1,
						Column: 1,
						Byte:   0,
					},
				},
			},
		},
		Inputs: inputsBlock,
	}

	evaluated, err := config.EvalComponent(evalctx, &component)
	if err != nil {
		t.Fatalf("error evaluating component: %s", err)
	}

	expected := config.Component{
		Name:   "test",
		Source: "test",
		Inputs: map[string]cty.Value{
			"test": cty.StringVal("value from inputs block"), // overridden by inputs block
		},
		Info: info.NewRange(rootdir, hhcl.Range{
			Filename: filepath.Join(rootdir, "test.hcl"),
			Start: hhcl.Pos{
				Line:   1,
				Column: 1,
				Byte:   0,
			},
		}),
	}

	if diff := cmp.Diff(expected, evaluated, cmpopts.IgnoreFields(config.Component{}, "Info", "Inputs")); diff != "" {
		t.Fatalf("component mismatch (-expected +evaluated):\n%s", diff)
	}

	// we cannot use cmp.Diff() because the types contains a lot of unexported fields.
	if len(expected.Inputs) != len(evaluated.Inputs) {
		t.Fatalf("inputs length mismatch (-expected +evaluated):\n%v", cmp.Diff(expected.Inputs, evaluated.Inputs))
	}

	for k, v := range expected.Inputs {
		got, ok := evaluated.Inputs[k]
		if !ok {
			t.Fatalf("input %q not found in evaluated", k)
		}

		// they are all strings for now
		gotStr := got.AsString()
		expStr := v.AsString()
		if gotStr != expStr {
			t.Fatalf("input %q mismatch (-expected +evaluated):\n%v", k, cmp.Diff(expStr, gotStr))
		}
	}
}
*/
