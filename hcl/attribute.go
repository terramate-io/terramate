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

package hcl

import (
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Attribute represents a parsed attribute.
type Attribute struct {
	origin string
	*hclsyntax.Attribute
}

// Attributes represents multiple parsed attributes.
// The attributes can be sorted lexicographically by their name.
type Attributes map[string]Attribute

// NewAttribute creates a new attribute given a parsed attribute and its origin.
func NewAttribute(origin string, val *hclsyntax.Attribute) Attribute {
	return Attribute{
		origin:    origin,
		Attribute: val,
	}
}

// Origin is the path of the file from where the attribute was parsed
// It is always an absolute path.
func (a Attribute) Origin() string {
	return a.origin
}

func (a Attributes) SortedList() AttributeSlice {
	var attrs AttributeSlice
	for _, val := range a {
		attrs = append(attrs, val)
	}
	sort.Sort(attrs)
	return attrs
}

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
