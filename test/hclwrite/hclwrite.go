// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package hclwrite aims to provide some facilities making it easier/safer
// to generate HCL code for testing purposes. It aims at:
//
// - Close to how HCL is written.
// - Provide formatted string representation.
// - Avoid issues when raw HCL strings are used on tests in general.
//
// It is not a replacement to hclwrite: https://pkg.go.dev/github.com/hashicorp/hcl/v2/hclwrite
// It is just easier/nicer to use on tests + circumvents some limitations like:
//
// - https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
//
// We are missing a way to build objects and lists with a similar syntax to blocks.
// For now we circumvent this with the AttributeValue function.
package hclwrite

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Block represents an HCL block.
type Block struct {
	name    string
	labels  []string
	hasexpr bool
	// Not cool to keep 2 copies of values but casting around
	// cty values is quite annoying, so this is a lazy solution.
	ctyvalues map[string]cty.Value
	contents  []string
}

// AddLabel add label on block.
func (b *Block) AddLabel(name string) {
	b.labels = append(b.labels, fmt.Sprintf("%q", name))
}

// AddExpr add expression on block. The expressions is kept as is on the
// final document.
func (b *Block) AddExpr(name string, expr string) {
	b.hasexpr = true
	b.addAttr(name, expr)
}

// AddNumberInt add number on block.
func (b *Block) AddNumberInt(name string, v int64) {
	b.ctyvalues[name] = cty.NumberIntVal(v)
	b.addAttr(name, v)
}

// AddString adds string on block.
func (b *Block) AddString(name string, v string) {
	b.ctyvalues[name] = cty.StringVal(v)
	b.addAttr(name, fmt.Sprintf("%q", v))
}

// AddBoolean adds boolean on block.
func (b *Block) AddBoolean(name string, v bool) {
	b.ctyvalues[name] = cty.BoolVal(v)
	b.addAttr(name, v)
}

// AddBlock adds a nested block on the block.
func (b *Block) AddBlock(child *Block) {
	b.contents = append(b.contents, child.String())
}

// AttributesValues gets all attributes that are evaluated
// values. Added expressions are ignored.
func (b *Block) AttributesValues() map[string]cty.Value {
	return b.ctyvalues
}

// HasExpressions returns true if block has any non-evaluated
// expressions.
func (b *Block) HasExpressions() bool {
	return b.hasexpr
}

// Build builds the given parent block by adding itself on it.
func (b *Block) Build(parent *Block) {
	parent.AddBlock(b)
}

// String returns a string representation of the block that should always be
// formatted HCL code.
func (b *Block) String() string {
	var code string

	if b.name != "" {
		code = b.name + strings.Join(b.labels, " ") + "{\n"
	}
	// Tried properly using hclwrite, it doesnt work well with expressions:
	// - https://stackoverflow.com/questions/67945463/how-to-use-hcl-write-to-set-expressions-with
	code += strings.Join(b.contents, "\n")

	if b.name != "" {
		if len(b.contents) > 0 {
			code += "\n"
		}
		code += "}"
	}
	return Format(code)
}

// BlockBuilder provides a general purpose way to build blocks.
type BlockBuilder interface {
	Build(*Block)
}

// BlockBuilderFunc an adapter to allow the use of ordinary functions as BlockBuilders.
type BlockBuilderFunc func(*Block)

// BuildBlock builds a block with the given name and N block builders.
func BuildBlock(name string, builders ...BlockBuilder) *Block {
	b := newBlock(name)
	for _, builder := range builders {
		builder.Build(b)
	}
	return b
}

// BuildHCL builds a root HCL document.
func BuildHCL(builders ...BlockBuilder) *Block {
	// Design is messed up since we only have blocks.
	// Would be better to have explicit body/blocks.
	return BuildBlock("", builders...)
}

// Labels creates a block builder that adds labels to block.
func Labels(labels ...string) BlockBuilder {
	return BlockBuilderFunc(func(g *Block) {
		for _, label := range labels {
			g.AddLabel(label)
		}
	})
}

// AttributeValue accepts an expr as the attribute value, similar to Expression,
// but will evaluate the expr and store the resulting value so
// it will be available as an attribute value instead of as an
// expression. If evaluation fails the test caller will fail.
//
// The evaluation is quite limited, only suitable for evaluating
// objects/lists/etc, but won't work with any references to
// namespaces of function calls (context for evaluation is always empty).
func AttributeValue(t *testing.T, name string, expr string) BlockBuilder {
	t.Helper()

	rawbody := name + " = " + expr
	parser := hclparse.NewParser()
	res, diags := parser.ParseHCL([]byte(rawbody), "")
	if diags.HasErrors() {
		t.Fatalf("hclwrite.Eval: cant parse %s: %v", rawbody, diags)
	}
	body := res.Body.(*hclsyntax.Body)

	val, diags := body.Attributes[name].Expr.Value(nil)
	if diags.HasErrors() {
		t.Fatalf("hclwrite.Eval: cant eval %s: %v", rawbody, diags)
	}

	return BlockBuilderFunc(func(g *Block) {
		// hacky way to get original string representation of composite types
		// but also have proper cty values that can be compared.
		g.ctyvalues[name] = val
		g.addAttr(name, expr)
	})
}

// Expression adds the attribute with the given name with the
// given expression, the expression won't be evaluated and this won't
// affect the attribute values of the block.
//
// The given expression will be added on the generated output verbatim.
func Expression(name string, expr string) BlockBuilder {
	return BlockBuilderFunc(func(g *Block) {
		g.AddExpr(name, expr)
	})
}

// String add a string attribute to the block.
func String(name string, val string) BlockBuilder {
	return BlockBuilderFunc(func(g *Block) {
		g.AddString(name, val)
	})
}

// Boolean add a boolean attribute to the block.
func Boolean(name string, val bool) BlockBuilder {
	return BlockBuilderFunc(func(g *Block) {
		g.AddBoolean(name, val)
	})
}

// NumberInt add a number attribute to the block.
func NumberInt(name string, val int64) BlockBuilder {
	return BlockBuilderFunc(func(g *Block) {
		g.AddNumberInt(name, val)
	})
}

// Format formats the given HCL code.
func Format(code string) string {
	return strings.Trim(string(hclwrite.Format([]byte(code))), "\n ")
}

// Build calls the underlying builder function to build the given block.
func (builder BlockBuilderFunc) Build(b *Block) {
	builder(b)
}

func (b *Block) addAttr(name string, val interface{}) {
	b.contents = append(b.contents, fmt.Sprintf("%s=%v", name, val))
}

func newBlock(name string) *Block {
	return &Block{
		name:      name,
		ctyvalues: map[string]cty.Value{},
	}
}
