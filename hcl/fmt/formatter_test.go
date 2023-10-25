// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fmt_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/hcl/fmt"
	"github.com/zclconf/go-cty/cty"
)

func TestFormatAttributes(t *testing.T) {
	t.Parallel()
	type testcase struct {
		name       string
		attributes map[string]cty.Value
		want       string
	}

	tcases := []testcase{
		{
			name:       "format empty attributes",
			attributes: map[string]cty.Value{},
		},
		{
			name:       "format nil attributes",
			attributes: nil,
		},
		{
			name: "format single str attribute",
			attributes: map[string]cty.Value{
				"str": cty.StringVal("value"),
			},
			want: "str = \"value\"",
		},
		{
			name: "format multiple attributes",
			attributes: map[string]cty.Value{
				"str":  cty.StringVal("value"),
				"num":  cty.NumberIntVal(666),
				"bool": cty.BoolVal(true),
			},
			want: "bool = true\nnum  = 666\nstr  = \"value\"",
		},
		{
			name: "format map attribute",
			attributes: map[string]cty.Value{
				"map": cty.MapVal(map[string]cty.Value{
					"key1": cty.StringVal("value1"),
					"key2": cty.StringVal("value2"),
				}),
			},
			want: `map = {
  key1 = "value1"
  key2 = "value2"
}`,
		},
		{
			name: "format object attribute",
			attributes: map[string]cty.Value{
				"obj": cty.ObjectVal(map[string]cty.Value{
					"key1": cty.StringVal("value1"),
					"key2": cty.StringVal("value2"),
				}),
			},
			want: `obj = {
  key1 = "value1"
  key2 = "value2"
}`,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := fmt.FormatAttributes(tcase.attributes)
			assert.EqualStrings(t, tcase.want, got)
		})
	}
}
