// Copyright 2023 Mineiros GmbH
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

// Package filter provides helpers for filtering objects.
package filter

import (
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// Operation is the binary logic operation (OR, AND).
type Operation int

// TagClause represents a tag filter clause.
type TagClause struct {
	// Op is the clause operation logic.
	Op Operation
	// Tag is the tag name if this is a leaf node.
	Tag string
	// Children is the list of children branches (if any)
	Children []TagClause
}

const (
	// EQ is the equal operation.
	EQ Operation = iota
	// AND is the and operation.
	AND
	// OR is the or operation.
	OR
)

const andSymbol = ":"
const orSymbol = ","

// MatchTags tells if the filters match the provided tags list.
func MatchTags(filters []string, tags []string) bool {
	filter, found := parseTagClauses(filters...)
	if !found {
		return false
	}
	return matchTags(filter, tags)
}

func matchTags(filter TagClause, tags []string) bool {
	index := tomap(tags)
	switch filter.Op {
	case EQ:
		return index[filter.Tag]
	case OR:
		for _, clause := range filter.Children {
			if matchTags(clause, tags) {
				return true
			}
		}
		return false
	case AND:
		for _, clause := range filter.Children {
			if !matchTags(clause, tags) {
				return false
			}
		}
		return true
	default:
		panic(errors.E(errors.ErrInternal, "unreachable"))
	}
}

func tomap(tags []string) map[string]bool {
	m := map[string]bool{}
	for _, t := range tags {
		m[t] = true
	}
	return m
}

func parseTagClauses(filters ...string) (TagClause, bool) {
	var clauses []TagClause
	for _, filter := range filters {
		if filter != "" {
			clauses = append(clauses, parseTagClause(filter))
		}
	}
	if len(clauses) == 0 {
		return TagClause{}, false
	}
	if len(clauses) == 1 {
		return clauses[0], true
	}

	return TagClause{
		Op:       OR,
		Children: clauses,
	}, true
}

// parseTagClause parses the tag-filter syntax defined below:
//
//	EXPR    = TAGNAME [ OP EXPR]
//	TAGNAME = <string>
//	OP      = ":" | ","
//
// Semantically, the `:` operation has precedence over `,`.
// Examples:
//
//	a:b,c 		-> (A&&B)||c
//	a,b:c,d	-> A||(B&&C)||d
func parseTagClause(filter string) TagClause {
	rootNode := TagClause{
		Op: OR,
	}
	orBranches := strings.Split(filter, orSymbol)
	for _, orNode := range orBranches {
		andNodes := strings.Split(orNode, andSymbol)
		if len(andNodes) == 1 {
			rootNode.Children = append(rootNode.Children, TagClause{
				Op:  EQ,
				Tag: andNodes[0],
			})
		} else {
			branch := TagClause{
				Op: AND,
			}
			for _, leaf := range andNodes {
				branch.Children = append(branch.Children, parseTagClause(leaf))
			}
			rootNode.Children = append(rootNode.Children, branch)
		}

	}
	if len(rootNode.Children) == 1 {
		// simplify
		rootNode = rootNode.Children[0]
	}
	return rootNode
}
