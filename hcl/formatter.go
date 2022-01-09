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
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// FormatAttributes will format a given attribute map to a string.
// Name of the attributes are the keys and the corresponding value is mapped.
// The formatted output will always be lexicographically sorted by the attribute name,
// so calling this function with the same map multiple times produces the same result.
func FormatAttributes(attrs map[string]cty.Value) string {
	if len(attrs) == 0 {
		return ""
	}

	logger := log.With().
		Str("action", "FormatAttributes()").
		Logger()

	logger.Trace().
		Msg("Create empty hcl file.")
	f := hclwrite.NewEmptyFile()
	body := f.Body()
	sortedAttrNames := make([]string, 0, len(attrs))

	for name := range attrs {
		sortedAttrNames = append(sortedAttrNames, name)
	}

	logger.Trace().
		Msg("Sort attributes.")
	sort.Strings(sortedAttrNames)

	logger.Trace().
		Msg("Set attribute values.")
	for _, name := range sortedAttrNames {
		body.SetAttributeValue(name, attrs[name])
	}

	return strings.Trim(string(f.Bytes()), "\n")
}
