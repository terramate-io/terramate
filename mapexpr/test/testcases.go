// Package test implements testcases and test helpers for dealing with map blocks.
package test

import (
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/hclwrite/hclutils"
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
			Name: "map wit unexpected map block",
			Block: mapBlock(
				labels("var"),
				expr("for_each", "[]"),
				expr("key", "element.new"),
				expr("value", "element.new"),
				mapBlock(),
			),
		},
	}
}

var (
	labels   = hclutils.Labels
	value    = hclutils.Value
	expr     = hclutils.Expr
	number   = hclutils.Number
	mapBlock = hclutils.Map
)
