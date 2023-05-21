// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fmt

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
