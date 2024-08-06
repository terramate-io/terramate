// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package sharing implements the loading of sharing related blocks.
package sharing

import (
	stdfmt "fmt"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/generate/genhcl"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
)

// File is a sharing backend generated file.
type File struct {
	magicCommentStyle genhcl.CommentStyle
	filename          string
	origin            info.Range
	body              string
	condition         bool
}

// PrepareFile prepares a sharing backend generated file.
func PrepareFile(root *config.Root, filename string, inputs config.Inputs, outputs config.Outputs) (File, error) {
	commentStyle := genhcl.CommentStyleFromConfig(root.Tree())
	gen := hclwrite.NewEmptyFile()
	body := gen.Body()
	var info info.Range
	for _, in := range inputs {
		if info.ToHCLRange().Empty() {
			info = in.Range
		}
		varBlock := hclwrite.NewBlock("variable", []string{in.Name})
		blockBody := varBlock.Body()
		blockBody.SetAttributeRaw("type", hclwrite.Tokens{
			{
				Type:  hclsyntax.TokenIdent,
				Bytes: []byte("string"),
			},
		})
		body.AppendBlock(varBlock)
	}
	for _, out := range outputs {
		if info.ToHCLRange().Empty() {
			info = out.Range
		}
		outBlock := hclwrite.NewBlock("output", []string{out.Name})
		blockBody := outBlock.Body()
		blockBody.SetAttributeRaw("value", ast.TokensForExpression(out.Value))
		body.AppendBlock(outBlock)
	}

	return File{
		magicCommentStyle: commentStyle,
		origin:            info,
		filename:          filename,
		condition:         (len(inputs) + len(outputs)) != 0,
		body:              string(hclwrite.Format(gen.Bytes())),
	}, nil
}

// Builtin returns true for sharing_backend related blocks.
func (f File) Builtin() bool { return true }

// Label of the original generate_hcl block.
func (f File) Label() string {
	return f.filename
}

// Asserts returns nil
func (f File) Asserts() []config.Assert {
	return nil
}

// Header returns the header of the generated HCL file.
func (f File) Header() string {
	return genhcl.Header(f.magicCommentStyle)
}

// Body returns a string representation of the HCL code
// or an empty string if the config itself is empty.
func (f File) Body() string {
	return string(f.body)
}

// Range returns the range information of the generate_file block.
func (f File) Range() info.Range {
	return f.origin
}

// Condition is true if there's any input or output to be generated.
func (f File) Condition() bool {
	return f.condition
}

// Context of the generate_hcl block.
func (f File) Context() string {
	return "stack" // always the case for sharing backend.
}

func (f File) String() string {
	return stdfmt.Sprintf("Generating file %q (condition %t) (body %q) (origin %q)",
		f.Label(), f.Condition(), f.Body(), f.Range().HostPath())
}
