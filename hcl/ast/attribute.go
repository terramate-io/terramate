// Copyright 2022 Mineiros GmbH
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

package ast

import (
	"sort"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Attribute represents a parsed attribute.
type Attribute struct {
	Origin string
	*hhcl.Attribute
}

// Attributes represents multiple parsed attributes.
type Attributes map[string]Attribute

// NewAttribute creates a new attribute given a parsed attribute and its origin.
func NewAttribute(origin string, val *hhcl.Attribute) Attribute {
	return Attribute{
		Origin:    origin,
		Attribute: val,
	}
}

// SortedList returns a sorted list of attributes from the map.
func (a Attributes) SortedList() AttributeSlice {
	var attrs AttributeSlice
	for _, val := range a {
		attrs = append(attrs, val)
	}
	sort.Sort(attrs)
	return attrs
}

// NewAttributes creates a map of Attributes from the raw hcl.Attributes.
func NewAttributes(origin string, rawAttrs hhcl.Attributes) Attributes {
	attrs := make(Attributes)
	for _, rawAttr := range rawAttrs {
		attrs[rawAttr.Name] = NewAttribute(origin, rawAttr)
	}
	return attrs
}

// AsHCLAttributes converts hclsyntax.Attributes to hcl.Attributes.
func AsHCLAttributes(syntaxAttrs hclsyntax.Attributes) hhcl.Attributes {
	attrs := make(hhcl.Attributes)
	for _, synAttr := range syntaxAttrs {
		attrs[synAttr.Name] = synAttr.AsHCLAttribute()
	}
	return attrs
}

// AttributeSlice is an sortable Attribute slice.
type AttributeSlice []Attribute

// Len returns the size of the attributes slice.
func (a AttributeSlice) Len() int {
	return len(a)
}

// Less returns true if i < j, false otherwise.
func (a AttributeSlice) Less(i, j int) bool {
	return a[i].Name < a[j].Name
}

// Swap swaps i and j.
func (a AttributeSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// SortRawAttributes sorts the raw attributes.
func SortRawAttributes(attrs hhcl.Attributes) []*hhcl.Attribute {
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}

	sort.Strings(names)
	sorted := make([]*hhcl.Attribute, len(names))
	for i, name := range names {
		sorted[i] = attrs[name]
	}

	return sorted
}
