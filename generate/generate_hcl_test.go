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

func expr(name string, expr string) hclwrite.BlockBuilder {
	return hclwrite.Expression(name, expr)
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

func terramate(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("terramate", builders...)
}

func exportAsLocals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("export_as_locals", builders...)
}
