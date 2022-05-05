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

package stack_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	tmstack "github.com/mineiros-io/terramate/stack"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoadFailsWithInvalidConfig(t *testing.T) {
	generateHCL := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_hcl", builders...)
	}
	generateFile := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_file", builders...)
	}
	stack := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("stack", builders...)
	}
	hcldoc := hclwrite.BuildHCL
	block := hclwrite.BuildBlock
	labels := hclwrite.Labels
	exprAttr := hclwrite.Expression
	strAttr := hclwrite.String

	tcases := map[string]fmt.Stringer{
		"generate_hcl no label": hcldoc(
			generateHCL(
				block("content"),
			),
		),
		"generate_hcl no content block": hcldoc(
			generateHCL(
				labels("test.tf"),
			),
		),
		"generate_hcl extra unknown attr": hcldoc(
			generateHCL(
				labels("test.tf"),
				block("content"),
				exprAttr("unrecognized", `"value"`),
			),
		),
		"generate_file no label": hcldoc(
			generateFile(
				strAttr("content", "test"),
			),
		),
		"generate_file no content": hcldoc(
			generateFile(
				labels("test.tf"),
			),
		),
		"generate_file extra unknown attr": hcldoc(
			generateFile(
				labels("test.tf"),
				strAttr("content", "value"),
				strAttr("unrecognized", "value"),
			),
		),
	}

	for testname, invalidConfig := range tcases {
		t.Run(testname, func(t *testing.T) {
			s := sandbox.New(t)

			stackEntry := s.CreateStack("stack")
			stackEntry.CreateConfig(invalidConfig.String() + "\n" + stack().String())

			_, err := tmstack.Load(s.RootDir(), stackEntry.Path())
			assert.IsError(t, err, errors.E(hcl.ErrTerramateSchema))
		})
	}
}
