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
	. "github.com/mineiros-io/terramate/test/hclwrite/hclutils"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoadFailsWithInvalidConfig(t *testing.T) {
	tcases := map[string]fmt.Stringer{
		"generate_hcl no label": Doc(
			GenerateHCL(
				Block("content"),
			),
		),
		"generate_hcl no content block": Doc(
			GenerateHCL(
				Labels("test.tf"),
			),
		),
		"generate_hcl extra unknown attr": Doc(
			GenerateHCL(
				Labels("test.tf"),
				Block("content"),
				Expr("unrecognized", `"value"`),
			),
		),
		"generate_file no label": Doc(
			GenerateFile(
				Str("content", "test"),
			),
		),
		"generate_file no content": Doc(
			GenerateFile(
				Labels("test.tf"),
			),
		),
		"generate_file extra unknown attr": Doc(
			GenerateFile(
				Labels("test.tf"),
				Str("content", "value"),
				Str("unrecognized", "value"),
			),
		),
	}

	for testname, invalidConfig := range tcases {
		t.Run(testname, func(t *testing.T) {
			s := sandbox.New(t)

			stackEntry := s.CreateStack("stack")
			stackEntry.CreateConfig(invalidConfig.String() + "\n" + Stack().String())

			_, err := tmstack.Load(s.RootDir(), stackEntry.Path())
			assert.IsError(t, err, errors.E(hcl.ErrTerramateSchema))
		})
	}
}
