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

package generate_test

import "github.com/mineiros-io/terramate/test/hclwrite"

// useful function aliases to build HCL documents

func hcldoc(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildHCL(builders...)
}

func generateHCL(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("generate_hcl", builders...)
}

func generateFile(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("generate_file", builders...)
}

func content(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("content", builders...)
}

func expr(name string, expr string) hclwrite.BlockBuilder {
	return hclwrite.Expression(name, expr)
}

func str(name string, val string) hclwrite.BlockBuilder {
	return hclwrite.String(name, val)
}

func number(name string, val int64) hclwrite.BlockBuilder {
	return hclwrite.NumberInt(name, val)
}

func boolean(name string, val bool) hclwrite.BlockBuilder {
	return hclwrite.Boolean(name, val)
}

func labels(labels ...string) hclwrite.BlockBuilder {
	return hclwrite.Labels(labels...)
}

func stack(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("stack", builders...)
}

func backend(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("backend", builders...)
}

func block(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock(name, builders...)
}

func globals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("globals", builders...)
}

func locals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("locals", builders...)
}

func terraform(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("terraform", builders...)
}

func importy(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("import", builders...)
}
