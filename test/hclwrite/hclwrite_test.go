package hclwrite_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mineiros-io/terramate/test/hclwrite"
)

func TestHCLWrite(t *testing.T) {
	type testcase struct {
		name  string
		block *hclwrite.Block
		want  string
	}

	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.NewBuilder(name, builders...)
	}
	labels := hclwrite.Labels
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name: "single block",
			block: block("test",
				str("str", "test"),
				number("num", 666),
				boolean("bool", true),
				expr("expr_a", "local.name"),
				expr("expr_b", "local.name"),
			),
			want: `
			  test {
			    expr_a = local.name
			    expr_b = local.name
			    bool   = true
			    num    = 666
			    str    = "test"
			  }
			`,
		},
		{
			name: "single block with one label",
			block: block("test",
				labels("label"),
				str("str", "labeltest"),
			),
			want: `
			  test "label" {
			    str    = "labeltest"
			  }
			`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			want := hclwrite.Format(tcase.want)
			got := tcase.block.String()

			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("got:\n%s", got)
				t.Errorf("want:\n%s", want)
				t.Error("diff:")
				t.Fatal(diff)
			}
		})
	}
}
