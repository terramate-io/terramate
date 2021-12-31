// Copyright 2021 Mineiros GmbH
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
		return hclwrite.BuildBlock(name, builders...)
	}
	labels := hclwrite.Labels
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	tcases := []testcase{
		{
			name:  "empty block",
			block: block("test"),
			want: `
			  test {
			  }
			`,
		},
		{
			name: "block with multiple attributes",
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
			name: "block with one label",
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
		{
			name: "empty block with one label",
			block: block("test",
				labels("label"),
			),
			want: `
			  test "label" {
			  }
			`,
		},
		{
			name: "block multiple labels",
			block: block("test",
				labels("label", "label2"),
				str("str", "labelstest"),
			),
			want: `
			  test "label" "label2" {
			    str    = "labelstest"
			  }
			`,
		},
		{
			name: "block nesting",
			block: block("test",
				str("str", "level1"),
				block("nested",
					str("str", "level2"),
					block("yet_more_nesting",
						str("str", "level3"),
					),
				),
			),
			want: `
			  test {
			    str = "level1"
			    nested {
			      str = "level2"
			      yet_more_nesting {
			        str = "level3"
			      }
			    }
			  }
			`,
		},
		{
			name: "block nesting with labels",
			block: block("test",
				labels("label"),
				str("str", "level1"),
				block("nested",
					labels("label1", "label2"),
					str("str", "level2"),
					block("yet_more_nesting",
						labels("label1", "label2", "label3"),
						str("str", "level3"),
					),
				),
			),
			want: `
			  test "label" {
			    str = "level1"
			    nested "label1" "label2" {
			      str = "level2"
			      yet_more_nesting "label1" "label2" "label3" {
			        str = "level3"
			      }
			    }
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
