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

// Package test implements testcases and test helpers for dealing with map blocks.
package test

import (
	"github.com/terramate-io/terramate/test/hclwrite"
	"github.com/terramate-io/terramate/test/hclwrite/hclutils"
)

// Testcase is a mapexpr test case.
type Testcase struct {
	Name  string
	Block *hclwrite.Block
}

// SchemaErrorTestcases returns test cases for schema errors.
func SchemaErrorTestcases() []Testcase {
	return []Testcase{
		{
			Name: "map with no label",
			Block: mapBlock(
				expr("for_each", `[]`),
				expr("key", `element.new`),
				expr("value", `element.new`),
			),
		},
		{
			Name: "map with no for_each",
			Block: mapBlock(
				labels("var"),
				expr("key", `element.new`),
				expr("value", `element.new`),
			),
		},
		{
			Name: "map with no key",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `[]`),
				expr("value", `element.new`),
			),
		},
		{
			Name: "map with no value",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `[]`),
				expr("key", `element.new`),
			),
		},
		{
			Name: "map with conflicting value",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `[]`),
				expr("key", `element.new`),
				expr("value", `element.new`),
				value(
					number("num", 1),
				),
			),
		},
		{
			Name: "map with multiple value blocks",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `[]`),
				expr("key", `element.new`),
				value(
					number("num", 1),
				),
				value(
					number("num2", 1),
				),
			),
		},
		{
			Name: "map with multiple value blocks",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `[]`),
				expr("key", "element.new"),
				value(
					expr("val1", `element.new`),
				),
				value(
					number("num", 1),
				),
			),
		},
		{
			Name: "map with unexpected map block",
			Block: mapBlock(
				labels("var"),
				expr("for_each", "[]"),
				expr("key", "element.new"),
				expr("value", "element.new"),
				mapBlock(),
			),
		},
		{
			Name: "nested map with conflicting map labels",
			Block: mapBlock(
				labels("var"),
				expr("for_each", `global.lst`),
				expr("key", "element.new"),

				value(
					str("some", "value"),
					mapBlock(
						labels("same_label"),
						expr("for_each", "global.lst"),
						expr("key", "element.new"),
						expr("value", "element.new"),
					),
					mapBlock(
						labels("same_label"),
						expr("for_each", "global.lst"),
						expr("key", "element.new"),
						expr("value", "element.new"),
					),
				),
			),
		},
	}
}

var (
	labels   = hclutils.Labels
	value    = hclutils.Value
	expr     = hclutils.Expr
	str      = hclutils.Str
	number   = hclutils.Number
	mapBlock = hclutils.Map
)
