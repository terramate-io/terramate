// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml

import (
	"github.com/terramate-io/terramate/errors"
)

const (
	apiVersion = "terramate.io/cli/v1"
)

// Error kinds for YAML decoding.
const (
	ErrSyntax errors.Kind = "syntax error"
	ErrSchema errors.Kind = "schema error"
)

// Seq is  YAML list.
type Seq[T any] []SeqItem[T]

// SeqItem is a list item.
type SeqItem[T any] struct {
	Value Attribute[T]
}

// Map is a YAML map.
// To preserve ordering, it is a list of key/value items.
type Map[T any] []MapItem[T]

// MapItem is a map item.
// HeadComment is stored in Key.HeadComment (comment above the key).
// LineComment is stored in Value.LineComment (end-of-line comment).
type MapItem[T any] struct {
	Key   Attribute[string]
	Value Attribute[T]
}

// Document is the common structure of YAML-encoded data object.
// Specific data objects (i.e. Bundle) can be used with Encode()/Decode(),
// which use Document as an intermediate representation.
type Document struct {
	APIVersion   Attribute[string]    `yaml:"apiVersion"`
	Kind         Attribute[string]    `yaml:"kind"`
	Metadata     Attribute[Map[any]]  `yaml:"metadata"`
	Spec         Attribute[Map[any]]  `yaml:"spec"`
	Environments *Attribute[Map[any]] `yaml:"environments,omitempty"`
}

// Attribute is a generic container with an optional source location for diagnostics reporting
// and optional YAML comments for round-trip preservation.
// Location is only used for decoding.
type Attribute[T any] struct {
	V           T
	Line        int
	Column      int
	HeadComment string
	LineComment string
}

// Attr is a helper function to create an Attribute.
func Attr[T any](v T, args ...any) Attribute[T] {
	var line, column int
	var headComment, lineComment string
	switch len(args) {
	case 0:
		break
	case 4:
		lineComment = args[3].(string)
		fallthrough
	case 3:
		headComment = args[2].(string)
		fallthrough
	case 2:
		line = args[0].(int)
		column = args[1].(int)

	default:
		panic("invalid args")
	}

	return Attribute[T]{V: v, Line: line, Column: column, HeadComment: headComment, LineComment: lineComment}
}

func attrToMapItem[T any](name string, attr Attribute[T]) MapItem[any] {
	return MapItem[any]{
		Key:   Attribute[string]{V: name, HeadComment: attr.HeadComment},
		Value: Attribute[any]{V: attr.V, LineComment: attr.LineComment},
	}
}

// NilAttr is a nil-valued attribute.
var NilAttr = Attr[any](nil)

func (a Attribute[T]) unwrapAttribute() (any, int, int) {
	return a.V, a.Line, a.Column
}

// TypeInfo defines common functions for type encoders and decoders.
type TypeInfo interface {
	APIVersion() string
	Kind() string
}
