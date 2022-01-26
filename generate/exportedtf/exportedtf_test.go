package exportedtf_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate/exportedtf"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestLoadExportedTerraform(t *testing.T) {
	type (
		hclconfig struct {
			path string
			add  fmt.Stringer
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    map[string]fmt.Stringer
			wantErr error
		}
	)

	exportAsTerraform := func(label string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		b := hclwrite.BuildBlock("export_as_terraform", builders...)
		b.AddLabel(label)
		return b
	}
	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name:  "no exported terraform",
			stack: "/stack",
		},
		{
			name:  "exported terraform on stack with single block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: exportAsTerraform("test",
						block("testblock",
							expr("string", "global.some_string"),
							expr("number", "global.some_number"),
							expr("bool", "global.some_bool"),
						),
					),
				},
			},
			want: map[string]fmt.Stringer{
				"test": block("testblock",
					str("string_local", "string"),
					number("number_local", 777),
					boolean("bool_local", true),
				),
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, config.Filename, cfg.add.String())
			}

			meta := stack.Meta()
			globals := s.LoadStackGlobals(meta)
			_, err := exportedtf.Load(s.RootDir(), meta, globals)
			assert.IsError(t, err, tcase.wantErr)

			// TODO(katcipis): check exported terraform
		})
	}
}
