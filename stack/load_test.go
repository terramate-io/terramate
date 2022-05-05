package stack_test

import (
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

	invalidConfigs := []string{
		hcldoc(
			generateHCL(
				exprAttr("undefined", "terramate.undefined"),
			),
			stack(),
		).String(),

		hcldoc(
			generateHCL(
				labels("test.tf"),
			),
			stack(),
		).String(),

		hcldoc(
			generateHCL(
				labels("test.tf"),
				block("content"),
				exprAttr("unrecognized", `"value"`),
			),
			stack(),
		).String(),

		hcldoc(
			generateFile(
				strAttr("content", "test"),
			),
			stack(),
		).String(),

		hcldoc(
			generateFile(
				labels("test.tf"),
			),
			stack(),
		).String(),

		hcldoc(
			generateFile(
				labels("test.tf"),
				strAttr("content", "value"),
				strAttr("unrecognized", "value"),
			),
			stack(),
		).String(),
	}

	for _, invalidConfig := range invalidConfigs {
		s := sandbox.New(t)

		stackEntry := s.CreateStack("stack")
		stackEntry.CreateConfig(invalidConfig)

		_, err := tmstack.Load(s.RootDir(), stackEntry.Path())
		assert.IsError(t, err, errors.E(hcl.ErrTerramateSchema))
	}
}
