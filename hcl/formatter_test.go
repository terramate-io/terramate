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

package hcl_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/zclconf/go-cty/cty"
)

func TestFormatAttributes(t *testing.T) {
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
			got := hcl.FormatAttributes(tcase.attributes)
			assert.EqualStrings(t, tcase.want, got)
		})
	}
}
