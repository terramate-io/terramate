// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hclutils

import (
	"bytes"
	"strings"
	"testing"

	"github.com/terramate-io/terramate/test/hclwrite"
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

// Script is a helper for a "script" block.
func Script(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("script", builders...)
}

// Input is a helper for an "input" block.
func Input(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("input", builders...)
}

// Output is a helper for an "output" block.
func Output(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("output", builders...)
}

// Command is a helper for a "command" attribute.
func Command(args ...string) hclwrite.BlockBuilder {
	expr := `["` + strings.Join(args, `","`) + `"]`
	return Expr("command", expr)
}

// Commands is a helper for a "commands" attribute.
func Commands(args ...[]string) hclwrite.BlockBuilder {
	buf := bytes.Buffer{}
	buf.WriteString("[")
	for i, arg := range args {
		buf.WriteString(`[` + strings.Join(arg, ",") + `]`)
		if i != len(args)-1 {
			buf.WriteString(",")
		}
	}
	buf.WriteString("]")
	return Expr("commands", buf.String())
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

// StackFilter is a helper for a "stack_filter" block.
func StackFilter(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("stack_filter", builders...)
}

// ProjectPaths is a helper for adding a "project_paths" attribute.
func ProjectPaths(paths ...string) hclwrite.BlockBuilder {
	expr := `["` + strings.Join(paths, `","`) + `"]`
	return hclwrite.Expression("project_paths", expr)
}

// RepositoryPaths is a helper for adding a "project_paths" attribute.
func RepositoryPaths(paths ...string) hclwrite.BlockBuilder {
	expr := `["` + strings.Join(paths, `","`) + `"]`
	return hclwrite.Expression("repository_paths", expr)
}

// Experiments is a helper for adding an `experiments` attribute.
func Experiments(names ...string) hclwrite.BlockBuilder {
	expr := `["` + strings.Join(names, `","`) + `"]`
	return hclwrite.Expression("experiments", expr)
}

// Lets is a helper for a "lets" block.
func Lets(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("lets", builders...)
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

// Map is a helper for a "map" block.
func Map(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("map", builders...)
}

// Value is a helper for a "value" block.
func Value(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("value", builders...)
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

// Assert is a helper for a "assert" block.
func Assert(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("assert", builders...)
}

// Trigger is a helper for a "trigger" block.
func Trigger(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return Block("trigger", builders...)
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
