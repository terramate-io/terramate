// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"path/filepath"
	"testing"

	hhcl "github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl/ast"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
)

func TestAttributeValueLiteralString(t *testing.T) {
	expr, diags := hclsyntax.ParseExpression([]byte(`"hello"`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := &hhcl.Attribute{Name: "name", Expr: expr}
	got := attributeValue(attr)
	if got.GetStringValue() != "hello" {
		t.Fatalf("expected string value, got %v", got)
	}
}

func TestAttributeValueExpressionFallback(t *testing.T) {
	expr, diags := hclsyntax.ParseExpression([]byte(`global.foo`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := &hhcl.Attribute{Name: "name", Expr: expr}
	got := attributeValue(attr)
	if got.GetExpressionText() == "" {
		t.Fatalf("expected expression text, got %v", got)
	}
	if got.Value.(*pb.AttributeValue_ExpressionText).ExpressionText != "global.foo" {
		t.Fatalf("unexpected expression text: %s", got.GetExpressionText())
	}
}

func TestAttributeValueLiteralBool(t *testing.T) {
	expr, diags := hclsyntax.ParseExpression([]byte(`true`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := &hhcl.Attribute{Name: "enabled", Expr: expr}
	got := attributeValue(attr)
	if got.GetBoolValue() != true {
		t.Fatalf("expected bool value true, got %v", got)
	}
}

func TestAttributeValueLiteralInt(t *testing.T) {
	expr, diags := hclsyntax.ParseExpression([]byte(`42`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := &hhcl.Attribute{Name: "count", Expr: expr}
	got := attributeValue(attr)
	if got.GetIntValue() != 42 {
		t.Fatalf("expected int value 42, got %v", got)
	}
}

func TestAttributeValueLiteralFloat(t *testing.T) {
	expr, diags := hclsyntax.ParseExpression([]byte(`1.5`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := &hhcl.Attribute{Name: "ratio", Expr: expr}
	got := attributeValue(attr)
	if got.GetFloatValue() != 1.5 {
		t.Fatalf("expected float value 1.5, got %v", got)
	}
}

func TestHCLOptionsFromSchemaBlockKinds(t *testing.T) {
	schemas := []*pb.HCLBlockSchema{
		{Name: "unmerged", Kind: pb.BlockKind_BLOCK_UNMERGED},
		{Name: "merged", Kind: pb.BlockKind_BLOCK_MERGED},
		{Name: "mergedlabels", Kind: pb.BlockKind_BLOCK_MERGED_LABELS},
		{Name: "unique", Kind: pb.BlockKind_BLOCK_UNIQUE},
	}
	opts := HCLOptionsFromSchema("demo", "/bin/true", schemas)
	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
}

func TestLabelsFromLabelType(t *testing.T) {
	lb, err := ast.NewLabelBlockType("globals", []string{"one", "two"})
	if err != nil {
		t.Fatalf("new label block type: %v", err)
	}
	labels := labelsFromLabelType(lb)
	if len(labels) != 2 || labels[0] != "one" || labels[1] != "two" {
		t.Fatalf("unexpected labels: %#v", labels)
	}

	empty := labelsFromLabelType(ast.NewEmptyLabelBlockType("globals"))
	if empty != nil {
		t.Fatalf("expected nil labels, got %#v", empty)
	}
}

func TestParsedBlockFromMergedNestedBlocks(t *testing.T) {
	rootDir := t.TempDir()
	expr, diags := hclsyntax.ParseExpression([]byte(`"ok"`), "test.hcl", hhcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse expression: %v", diags)
	}
	attr := ast.NewAttribute(rootDir, &hhcl.Attribute{
		Name:  "name",
		Expr:  expr,
		Range: hhcl.Range{Filename: filepath.Join(rootDir, "test.hcl")},
	})

	parent := ast.NewMergedBlock("parent", nil)
	parent.Attributes["name"] = attr

	labelType, err := ast.NewLabelBlockType("child", []string{"a", "b"})
	if err != nil {
		t.Fatalf("new label block type: %v", err)
	}
	child := ast.NewMergedBlock("child", nil)
	parent.Blocks[labelType] = child

	parsed := parsedBlockFromMerged(parent)
	if parsed.BlockType != "parent" {
		t.Fatalf("unexpected block type: %s", parsed.BlockType)
	}
	if parsed.Attributes["name"].GetStringValue() != "ok" {
		t.Fatalf("unexpected attribute value: %v", parsed.Attributes["name"])
	}
	if len(parsed.NestedBlocks) != 1 {
		t.Fatalf("expected one nested block, got %d", len(parsed.NestedBlocks))
	}
	nested := parsed.NestedBlocks[0]
	if nested.BlockType != "child" {
		t.Fatalf("unexpected nested block type: %s", nested.BlockType)
	}
	if len(nested.Labels) != 2 || nested.Labels[0] != "a" || nested.Labels[1] != "b" {
		t.Fatalf("unexpected nested labels: %#v", nested.Labels)
	}
}
