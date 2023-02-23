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

	"github.com/mineiros-io/terramate/config/tag"
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
	EQ Operation = iota + 1
	// NEQ is the not-equal operation.
	NEQ
	// AND is the and operation.
	AND
	// OR is the or operation.
	OR
)

const (
	andSymbol = ":"
	orSymbol  = ","
	neqSymbol = '~'
)

// IsEmpty tells if clause is empty
func (t TagClause) IsEmpty() bool {
	return t.Op == 0
}

// MatchTagsFrom tells if the filters match the provided tags list.
func MatchTagsFrom(filters []string, tags []string) (bool, error) {
	filter, found, err := ParseTagClauses(filters...)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return MatchTags(filter, tags), nil
}

// MatchTags tells if the filter matches the provided tags list.
func MatchTags(filter TagClause, tags []string) bool {
	index := tomap(tags)
	switch filter.Op {
	case EQ:
		return index[filter.Tag]
	case NEQ:
		return !index[filter.Tag]
	case OR:
		for _, clause := range filter.Children {
			if MatchTags(clause, tags) {
				return true
			}
		}
		return false
	case AND:
		for _, clause := range filter.Children {
			if !MatchTags(clause, tags) {
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

// ParseTagClauses parses the list of filters provided into a [TagClause] matcher.
// It returns a boolean telling if the clauses are not empty.
func ParseTagClauses(filters ...string) (TagClause, bool, error) {
	for _, filter := range filters {
		for _, orClause := range strings.Split(filter, ",") {
			for _, andClause := range strings.Split(orClause, ":") {
				err := tag.Validate(andClause)
				if err != nil {
					return TagClause{}, false, err
				}
			}
		}
	}
	return parseInternalTagClauses(filters...)
}

func parseInternalTagClauses(filters ...string) (TagClause, bool, error) {
	var clauses []TagClause
	for _, filter := range filters {
		if filter != "" {
			clause, err := parseTagClause(filter)
			if err != nil {
				return TagClause{}, true, err
			}
			clauses = append(clauses, clause)
		}
	}
	if len(clauses) == 0 {
		return TagClause{}, false, nil
	}
	if len(clauses) == 1 {
		return clauses[0], true, nil
	}

	return TagClause{
		Op:       OR,
		Children: clauses,
	}, true, nil
}

// parseTagClause parses the internal tag-filter (simplified) syntax defined
// below:
//
//	EXPR    = [ UNOP ] TAGNAME [ OP EXPR]
//	TAGNAME = <string>
//	BINOP   = ":" | ","
//	UNOP    = "~"
//
// Semantically, the `:` operation has precedence over `,`.
// Examples:
//
//	a:b,c 		-> (a&&b)||c
//	a,b:c,d	    -> a||(b&&c)||d
//
// Inequality examples:
//
//	~a          -> !a
//	a,~b        -> a||!b
//	~a:~b       -> !a&&!b
//
// This syntax is only used internally by Terramate.
// For the public syntax, see the spec at the link below:
// https://github.com/mineiros-io/terramate/blob/main/docs/tag-filter.md#filter-grammar
func parseTagClause(filter string) (TagClause, error) {
	rootNode := TagClause{
		Op: OR,
	}
	orBranches := strings.Split(filter, orSymbol)
	for _, orNode := range orBranches {
		andNodes := strings.Split(orNode, andSymbol)
		if len(andNodes) == 1 {
			op := EQ
			tagname := andNodes[0]
			if tagname[0] == neqSymbol {
				tagname = tagname[1:]
				op = NEQ
			}
			err := tag.Validate(tagname)
			if err != nil {
				return TagClause{}, err
			}

			rootNode.Children = append(rootNode.Children, TagClause{
				Op:  op,
				Tag: tagname,
			})
		} else {
			branch := TagClause{
				Op: AND,
			}
			for _, leaf := range andNodes {
				clause, err := parseTagClause(leaf)
				if err != nil {
					return TagClause{}, err
				}
				branch.Children = append(branch.Children, clause)
			}
			rootNode.Children = append(rootNode.Children, branch)
		}

	}
	if len(rootNode.Children) == 1 {
		// simplify
		rootNode = rootNode.Children[0]
	}
	return rootNode, nil
}
