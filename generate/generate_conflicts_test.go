package generate_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/generate"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestFilenameConflictsOnGeneration(t *testing.T) {
	// This test checks for how different code generation strategies
	// fail if they have conflicting filename configurations.
	// Like 2 different strategies that will write to the same filename.
	type testcase struct {
		name       string
		layout     []string
		configs    []hclconfig
		workingDir string
		want       error
	}

	// gen instead of generate because name conflicts with generate pkg
	gen := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate", builders...)
	}
	// cfg instead of config because name conflicts with config pkg
	cfg := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("config", builders...)
	}

	tcases := []testcase{
		{
			name:   "export_as_locals conflicting with generate_hcl",
			layout: []string{"stacks/stack"},
			configs: []hclconfig{
				{
					path: "stacks",
					add: hcldoc(
						terramate(cfg(gen(
							str("locals_filename", "file.tf"),
						))),
						exportAsLocals(
							str("test", "val"),
						),
						generateHCL(
							labels("file.tf"),
							block("test"),
						),
					),
				},
			},
			want: generate.ErrConflictingConfig,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			s.BuildTree(tcase.layout)

			for _, cfg := range tcase.configs {
				cfg.Append(t, s.RootDir())
			}

			workingDir := filepath.Join(s.RootDir(), tcase.workingDir)
			err := generate.Do(s.RootDir(), workingDir)
			assert.IsError(t, err, tcase.want)
		})
	}
}
