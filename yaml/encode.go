// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package yaml

import (
	"io"

	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/hcl/ast"

	"go.yaml.in/yaml/v4"
)

// DocumentEncoder defines functions to transform a data object into its Document representation.
type DocumentEncoder interface {
	TypeInfo
	EncodeMetadata() (Map[any], error)
	EncodeSpec() (Map[any], error)
	EncodeEnvs() (Map[any], error)
}

// StructEncoder is the interface for structs that can be encoded into a Map.
type StructEncoder interface {
	EncodeStruct() (Map[any], error)
}

// Encode is a helper function that transforms a data object implementing the Encoder interface
// into a Document and then write it as raw YAML data to w.
func Encode(e DocumentEncoder, w io.Writer) error {
	md, err := e.EncodeMetadata()
	if err != nil {
		return err
	}
	spec, err := e.EncodeSpec()
	if err != nil {
		return err
	}
	envs, err := e.EncodeEnvs()
	if err != nil {
		return err
	}
	doc := Document{
		APIVersion: Attr(e.APIVersion()),
		Kind:       Attr(e.Kind()),
		Metadata:   Attr(md),
		Spec:       Attr(spec),
	}
	if len(envs) > 0 {
		t := Attr(envs)
		doc.Environments = &t
	}

	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)

	if err := encoder.Encode(&doc); err != nil {
		return err
	}
	return nil
}

// MarshalYAML for sequences.
// Will return a YAML sequence node with comments preserved.
func (v Seq[T]) MarshalYAML() (any, error) {
	node := yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: make([]*yaml.Node, len(v)),
	}
	for i, elem := range v {
		valNode, err := encodeValue(elem.Value.V)
		if err != nil {
			return nil, err
		}
		valNode.HeadComment = elem.Value.HeadComment
		valNode.LineComment = elem.Value.LineComment
		node.Content[i] = valNode
	}
	return &node, nil
}

// MarshalYAML for maps.
// Will return a YAML mapping node with comments preserved.
func (v Map[T]) MarshalYAML() (any, error) {
	node := yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: make([]*yaml.Node, len(v)*2),
	}
	for i, elem := range v {
		offset := i * 2
		var keyNode yaml.Node
		if err := keyNode.Encode(elem.Key.V); err != nil {
			return nil, err
		}
		valNode, err := encodeValue(elem.Value.V)
		if err != nil {
			return nil, err
		}

		keyNode.HeadComment = elem.Key.HeadComment
		valNode.LineComment = elem.Value.LineComment

		node.Content[offset] = &keyNode
		node.Content[offset+1] = valNode
	}
	return &node, nil
}

// MarshalYAML for attributes.
func (in Attribute[T]) MarshalYAML() (any, error) {
	return in.V, nil
}

// encodeValue writes our data model back into a YAML nodes.
// Map, Seq will be turned into mapping/sequence nodes with comments.
// HCL expressions will be tagged with !hcl.
// Any other YAML supported values use the default encoder.
func encodeValue(in any) (*yaml.Node, error) {
	switch v := in.(type) {
	case *yaml.Node:
		// Break infinite recursion.
		return v, nil
	case hclsyntax.Expression:
		exprStr := ast.TokensForExpression(v).Bytes()

		node := yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!hcl",
			Value: string(exprStr),
		}
		return &node, nil
	case yaml.Marshaler:
		result, err := v.MarshalYAML()
		if err != nil {
			return nil, err
		}
		return result.(*yaml.Node), nil
	default:
		var node yaml.Node
		if err := node.Encode(&v); err != nil {
			return nil, err
		}
		return &node, nil
	}
}

// encodeMapOfStructs transforms a map of pointers to structs into a Map[any].
func encodeMapOfStructs[T StructEncoder](obj Attribute[Map[T]]) (Attribute[Map[any]], error) {
	ret := make(Map[any], len(obj.V))
	for i, mapItem := range obj.V {
		encoded, err := mapItem.Value.V.EncodeStruct()
		if err != nil {
			return Attribute[Map[any]]{}, err
		}
		ret[i] = MapItem[any]{
			Key:   mapItem.Key,
			Value: Attribute[any]{V: encoded, LineComment: mapItem.Value.LineComment},
		}
	}
	return Attribute[Map[any]]{
		V:           ret,
		HeadComment: obj.HeadComment,
		LineComment: obj.LineComment,
	}, nil
}
