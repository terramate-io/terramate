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

package e2etest

import "github.com/mineiros-io/terramate/test/hclwrite"

var (
	str     = hclwrite.String
	number  = hclwrite.NumberInt
	boolean = hclwrite.Boolean
	labels  = hclwrite.Labels
	expr    = hclwrite.Expression
)

func terramate(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("terramate", builders...)
}

func config(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("config", builders...)
}

func generateHCL(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("generate_hcl", builders...)
}

func content(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("content", builders...)
}

func globals(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
	return hclwrite.BuildBlock("globals", builders...)
}
