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

package hcl

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// FormatAttributes will format a given attribute map to a string.
// Name of the attributes are the keys and the corresponding value is mapped.
func FormatAttributes(attrs map[string]cty.Value) string {
	f := hclwrite.NewEmptyFile()
	body := f.Body()

	for name, value := range attrs {
		body.SetAttributeValue(name, value)
	}

	return strings.Trim(string(f.Bytes()), "\n")
}
