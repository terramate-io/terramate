// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml

import (
	"fmt"
	"io"

	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/errors"

	"go.yaml.in/yaml/v4"
)

// DocumentDecoder defines functions to load a data object from its Document representation.
type DocumentDecoder interface {
	TypeInfo
	DecodeMetadata(Attribute[Map[any]]) error
	DecodeSpec(Attribute[Map[any]]) error
	DecodeEnvs(Attribute[Map[any]]) error
}

// StructDecoder defines functions to decode a struct from a Map.
type StructDecoder interface {
	DecodeStruct(Attribute[Map[any]]) error
}

// Decode is a helper function that reads raw YAML data from r and
// decodes it into data object implementing the DocumentDecoder interface.
func Decode(r io.Reader, dec DocumentDecoder) error {
	var doc Document
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		parserErr := &yaml.ParserError{}
		if errors.As(err, &parserErr) {
			return Error{
				Err:  err,
				Line: parserErr.Line,
			}
		}
		return err
	}
	if doc.APIVersion.V != dec.APIVersion() {
		return newAttrErrorf(doc.APIVersion, "expected apiVersion %q, got %q", dec.APIVersion(), doc.APIVersion.V)
	}
	if doc.Kind.V != dec.Kind() {
		return newAttrErrorf(doc.Kind, "expected kind %q, got %q", dec.Kind(), doc.Kind.V)
	}

	if err := dec.DecodeMetadata(doc.Metadata); err != nil {
		return err
	}
	if err := dec.DecodeSpec(doc.Spec); err != nil {
		return err
	}
	if doc.Environments != nil {
		if err := dec.DecodeEnvs(*doc.Environments); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalYAML for sequences. Requieres sequence node.
func (out *Seq[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!seq" {
		dec, err := decodeValue(node)
		if err != nil {
			return err
		}
		if conv, ok := dec.V.(Seq[T]); ok {
			*out = conv
			return nil
		}
	}
	return Error{Err: fmt.Errorf("expected sequence"), Line: node.Line, Column: node.Column}
}

// UnmarshalYAML for maps. Requieres mapping node.
func (out *Map[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!map" {
		dec, err := decodeValue(node)
		if err != nil {
			return err
		}
		if conv, ok := dec.V.(Map[T]); ok {
			*out = conv
			return nil
		}
	}
	return Error{Err: fmt.Errorf("expected map"), Line: node.Line, Column: node.Column}
}

// UnmarshalYAML for attributes.
func (out *Attribute[T]) UnmarshalYAML(node *yaml.Node) error {
	dec, err := decodeValue(node)
	if err != nil {
		return err
	}
	if conv, ok := dec.V.(T); ok {
		*out = Attr(conv, dec.Line, dec.Column)
		return nil
	}
	var dummy T
	return newAttrErrorf(dec, "expected attribute of type %T", dummy)
}

// decodeValues transforms a YAML node hierarchy into our structure, consisting of
// Map, Seq, HCL expressions, and any other type that supports YAML marshalling.
// It is called recursively to preserve nested Map/Seq with comments,
// instead of turning them into Go maps or slices.
func decodeValue(node *yaml.Node) (Attribute[any], error) {
	switch node.Tag {
	case "!!seq":
		v := make(Seq[any], len(node.Content))
		for i, seqNode := range node.Content {
			seqVal, err := decodeValue(seqNode)
			if err != nil {
				return NilAttr, err
			}
			seqVal.HeadComment = seqNode.HeadComment
			seqVal.LineComment = seqNode.LineComment
			v[i] = SeqItem[any]{
				Value: seqVal,
			}
		}
		return Attr[any](v, node.Line, node.Column), nil
	case "!!map":
		v := make(Map[any], len(node.Content)/2)
		for i := range v {
			offset := i * 2
			var itemKey string

			keyNode := node.Content[offset]
			valNode := node.Content[offset+1]

			if err := keyNode.Decode(&itemKey); err != nil {
				return NilAttr, err
			}
			itemVal, err := decodeValue(valNode)
			if err != nil {
				return NilAttr, err
			}
			itemVal.LineComment = valNode.LineComment
			v[i] = MapItem[any]{
				Key:   Attribute[string]{V: itemKey, Line: keyNode.Line, Column: keyNode.Column, HeadComment: keyNode.HeadComment},
				Value: itemVal,
			}
		}
		return Attr[any](v, node.Line, node.Column), nil
	case "!hcl":
		// Column will be wrong.
		parsed, diag := hclsyntax.ParseExpression([]byte(node.Value), "<!hcl expression>", hcl.Pos{Line: node.Line})
		if diag.HasErrors() {
			return NilAttr, Error{Err: errors.E(diag, "HCL parse error"), Line: node.Line, Column: node.Column}
		}
		return Attr[any](parsed, node.Line, node.Column), nil
	default:
		var v any
		if err := node.Decode(&v); err != nil {
			return NilAttr, Error{Err: err, Line: node.Line, Column: node.Column}
		}
		return Attr(v, node.Line, node.Column), nil
	}
}

func newAttrError[T any](a Attribute[T], err error) Error {
	return Error{
		Err:    err,
		Line:   a.Line,
		Column: a.Column,
	}
}

func newAttrErrorf[T any](a Attribute[T], format string, args ...any) Error {
	return newAttrError(a, fmt.Errorf(format, args...))
}

func decodeMapOfStructs[T any, PT interface {
	*T
	StructDecoder
}](obj Attribute[Map[any]]) (Attribute[Map[PT]], error) {
	var ret Map[PT]
	for _, mapItem := range obj.V {
		switch o := mapItem.Value.V.(type) {
		case Map[any]:
			v := PT(new(T))
			if err := v.DecodeStruct(Attr(o, mapItem.Key.Line, mapItem.Key.Column)); err != nil {
				return Attribute[Map[PT]]{}, err
			}
			ret = append(ret, MapItem[PT]{
				Key:   mapItem.Key,
				Value: Attribute[PT]{V: v, Line: mapItem.Key.Line, Column: mapItem.Key.Column, LineComment: mapItem.Value.LineComment},
			})
		case nil:
			ret = append(ret, MapItem[PT]{
				Key: mapItem.Key,
			})
		default:
			return Attribute[Map[PT]]{}, newAttrErrorf(mapItem.Value, "item must be a map, was %T", o)
		}
	}
	return Attr(ret, obj.Line, obj.Column), nil
}
