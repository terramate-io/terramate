// Copyright 2022 Mineiros GmbH
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

package hclutils

import (
	"testing"

	"github.com/mineiros-io/terramate/test/hclwrite"
)

// useful function aliases to build HCL documents

// Doc is a helper for a HCL document.
func Doc(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildHCL(builders...)
}

// Terramate is a helper for a "terramate" block.
func Terramate(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("terramate", builders...)
}

// Config is a helper for a "config" block.
func Config(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("config", builders...)
}

// Run is a helper for a "run" block.
func Run(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("run", builders...)
}

// Vendor is a helper for a "vendor" block.
func Vendor(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("vendor", builders...)
}

// Manifest is a helper for a "manifest" block.
func Manifest(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("manifest", builders...)
}

// Default is a helper for a "default" block.
func Default(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("default", builders...)
}

// Env is a helper for a "env" block.
func Env(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("env", builders...)
}

// GenerateHCL is a helper for a "generate_hcl" block.
func GenerateHCL(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("generate_hcl", builders...)
}

// Variable is a helper for a "generate_hcl" block.
func Variable(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("variable", builders...)
}

// TmDynamic is a helper for a "generate_hcl" block.
func TmDynamic(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("tm_dynamic", builders...)
}

// GenerateFile is a helper for a "generate_file" block.
func GenerateFile(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("generate_file", builders...)
}

// Content is a helper for a "content" block.
func Content(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("content", builders...)
}

// Expr is a helper for a HCL expression.
func Expr(name string, expr string) hclwrite.BlockBuilder {
	return hclwrite.Expression(name, expr)
}

// Str is a helper for a string attribute.
func Str(name string, val string) hclwrite.BlockBuilder {
	return hclwrite.String(name, val)
}

// Number is a helper for a number attribute.
func Number(name string, val int64) hclwrite.BlockBuilder {
	return hclwrite.NumberInt(name, val)
}

// Bool is a helper for a boolean attribute.
func Bool(name string, val bool) hclwrite.BlockBuilder {
	return hclwrite.Boolean(name, val)
}

// Labels is a helper for adding labels to a block.
func Labels(labels ...string) hclwrite.BlockBuilder {
	return hclwrite.Labels(labels...)
}

// Backend is a helper for a "backend" block.
func Backend(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("backend", builders...)
}

// Block is a helper for creating arbitrary blocks of specified name/type.
func Block(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock(name, builders...)
}

// Globals is a helper for a "globals" block.
func Globals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("globals", builders...)
}

// Stack is a helper for a "stack" block.
func Stack(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("stack", builders...)
}

// Locals is a helper for a "locals" block.
func Locals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("locals", builders...)
}

// Terraform is a helper for a "terraform" block.
func Terraform(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("terraform", builders...)
}

// Module is a helper for a "module" block.
func Module(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("module", builders...)
}

// Import is a helper for an "import" block.
func Import(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("import", builders...)
}

// EvalExpr accepts an expr as the attribute value, similar to Expr,
// but will evaluate the expr and store the resulting value so
// it will be available as an attribute value instead of as an
// expression. If evaluation fails the test caller will fail.
//
// The evaluation is quite limited, only suitable for evaluating
// objects/lists/etc, but won't work with any references to
// namespaces except default Terraform function calls.
func EvalExpr(t *testing.T, name string, expr string) hclwrite.BlockBuilder {
	t.Helper()
	return hclwrite.AttributeValue(t, name, expr)
}
