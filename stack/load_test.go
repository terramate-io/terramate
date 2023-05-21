// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package stack_test

import (
	"fmt"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	. "github.com/terramate-io/terramate/test/hclwrite/hclutils"
	"github.com/terramate-io/terramate/test/sandbox"
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

			_, err := config.LoadTree(s.RootDir(), stackEntry.Path())
			assert.IsError(t, err, errors.E(hcl.ErrTerramateSchema))
		})
	}
}
