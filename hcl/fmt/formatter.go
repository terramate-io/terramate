// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package fmt

import (
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
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

	f := hclwrite.NewEmptyFile()
	body := f.Body()
	sortedAttrNames := make([]string, 0, len(attrs))

	for name := range attrs {
		sortedAttrNames = append(sortedAttrNames, name)
	}

	sort.Strings(sortedAttrNames)

	for _, name := range sortedAttrNames {
		body.SetAttributeValue(name, attrs[name])
	}

	return strings.Trim(string(f.Bytes()), "\n")
}
