// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package yaml provides YAML encoding and decoding for Terramate data objects.
package yaml

import (
	"fmt"

	"github.com/google/uuid"
)

// BundleInstance data object.
type BundleInstance struct {
	Name Attribute[string]
	UUID Attribute[string]

	// Source can be a string, or an HCL expression.
	Source       Attribute[any]
	Inputs       Attribute[Map[any]]
	Environments Attribute[Map[*BundleEnvironment]]
}

// BundleEnvironment represents environment-specific overrides for a bundle instance.
type BundleEnvironment struct {
	Source Attribute[any]
	Inputs Attribute[Map[any]]
}

// APIVersion returns the API version for bundle instances.
func (*BundleInstance) APIVersion() string {
	return apiVersion
}

// Kind returns the document kind for bundle instances.
func (*BundleInstance) Kind() string {
	return "BundleInstance"
}

// EncodeMetadata encodes the bundle instance metadata into a Map.
func (b *BundleInstance) EncodeMetadata() (Map[any], error) {
	enc := Map[any]{
		attrToMapItem("name", b.Name),
	}
	if b.UUID.V != "" {
		enc = append(enc, attrToMapItem("uuid", b.UUID))
	}
	return enc, nil
}

// EncodeSpec encodes the bundle instance spec into a Map.
func (b *BundleInstance) EncodeSpec() (Map[any], error) {
	r := Map[any]{
		attrToMapItem("source", b.Source),
	}
	if len(b.Inputs.V) > 0 {
		r = append(r, attrToMapItem("inputs", b.Inputs))
	}
	return r, nil
}

// EncodeEnvs encodes the bundle instance environments into a Map.
func (b *BundleInstance) EncodeEnvs() (Map[any], error) {
	if len(b.Environments.V) == 0 {
		return Map[any]{}, nil
	}
	envMap, err := encodeMapOfStructs(b.Environments)
	if err != nil {
		return nil, err
	}
	return envMap.V, nil
}

// DecodeMetadata decodes metadata from a Map into the bundle instance.
func (b *BundleInstance) DecodeMetadata(md Attribute[Map[any]]) error {
	for _, mapItem := range md.V {
		switch mapItem.Key.V {
		case "name":
			var err error
			b.Name, err = decodeName(mapItem.Value)
			if err != nil {
				return err
			}
		case "uuid":
			var err error
			b.UUID, err = decodeUUID(mapItem.Value)
			if err != nil {
				return err
			}
		default:
			return newAttrErrorf(mapItem.Key, "unexpected attribute: metadata.%s. must be one of [name, uuid]", mapItem.Key.V)
		}
	}
	if b.Name.V == "" {
		return newAttrErrorf(md, "metadata.name is missing")
	}
	return nil
}

// DecodeSpec decodes a spec from a Map into the bundle instance.
func (b *BundleInstance) DecodeSpec(spec Attribute[Map[any]]) error {
	for _, mapItem := range spec.V {
		switch mapItem.Key.V {
		case "source":
			switch mapItem.Value.V.(type) {
			case string:
				b.Source = mapItem.Value
				b.Source.HeadComment = mapItem.Key.HeadComment
			default:
				return newAttrErrorf(mapItem.Value, "spec.source must be a string")
			}
		case "inputs":
			switch inputs := mapItem.Value.V.(type) {
			case Map[any]:
				b.Inputs = Attr(inputs, mapItem.Key.Line, mapItem.Key.Column, mapItem.Key.HeadComment, mapItem.Value.LineComment)
			default:
				return newAttrErrorf(mapItem.Value, "spec.inputs must be a map")
			}
		default:
			return newAttrErrorf(mapItem.Key, "unexpected attribute: spec.%s. must be one of [source, inputs]", mapItem.Key.V)
		}
	}
	if b.Source.V == nil {
		return fmt.Errorf("spec.source is missing")
	}
	return nil
}

// DecodeEnvs decodes environments from a Map into the bundle instance.
func (b *BundleInstance) DecodeEnvs(envs Attribute[Map[any]]) error {
	var err error
	b.Environments, err = decodeMapOfStructs[BundleEnvironment](envs)
	if err != nil {
		return err
	}

	return nil
}

func decodeName(v Attribute[any]) (Attribute[string], error) {
	s, ok := v.V.(string)
	if !ok {
		return Attr(""), newAttrErrorf(v, "metadata.name must be of type string")
	}
	if s == "" {
		return Attr(""), newAttrErrorf(v, "metadata.name must not be empty")
	}
	if len(s) > 255 {
		return Attr(""), newAttrErrorf(v, "metadata.name must be no longer than 255 characters")
	}

	for i, r := range s {
		switch i {
		case 0:
			if !isLowerAlpha(r) {
				return Attr(""), newAttrErrorf(v, "metadata.name must start a with lowercase character ([a-z])")
			}
		case len(s) - 1:
			if !isLowerAlnum(r) {
				return Attr(""), newAttrErrorf(v, "metadata.name must end with a lowercase alphanumeric character ([0-9a-z])")
			}
		default:
			if !isLowerAlnum(r) && r != '-' && r != '_' {
				return Attr(""), newAttrErrorf(v, "metadata.name may only contain lowercase alphanumeric characters, '-', or '_'")
			}
		}
	}
	return Attr(s, v.Line, v.Column), nil
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLowerAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z')
}
func isLowerAlnum(r rune) bool {
	return isLowerAlpha(r) || isDigit(r)
}

func decodeUUID(v Attribute[any]) (Attribute[string], error) {
	s, ok := v.V.(string)
	if !ok {
		return Attr(""), newAttrErrorf(v, "metadata.uuid must be a string")
	}
	if s == "" {
		return Attr(""), newAttrErrorf(v, "metadata.uuid must not be empty")
	}
	r, err := uuid.Parse(s)
	if err != nil {
		return Attr(""), newAttrErrorf(v, "metadata.uuid must be a valid UUID")
	}
	return Attr(r.String(), v.Line, v.Column), nil
}

// EncodeStruct encodes the environment into a Map.
func (e *BundleEnvironment) EncodeStruct() (Map[any], error) {
	r := Map[any]{}
	if e.Source.V != nil {
		r = append(r, attrToMapItem("source", e.Source))
	}
	if len(e.Inputs.V) > 0 {
		r = append(r, MapItem[any]{
			Key:   Attribute[string]{V: "inputs", HeadComment: e.Inputs.HeadComment},
			Value: Attribute[any]{V: e.Inputs.V, LineComment: e.Inputs.LineComment},
		})
	}
	return r, nil
}

// DecodeStruct decodes a Map into the environment.
func (e *BundleEnvironment) DecodeStruct(data Attribute[Map[any]]) error {
	for _, mapItem := range data.V {
		switch mapItem.Key.V {
		case "source":
			switch mapItem.Value.V.(type) {
			case string:
				e.Source = mapItem.Value
				e.Source.HeadComment = mapItem.Key.HeadComment
			default:
				return newAttrErrorf(mapItem.Value, "source must be a string")
			}
		case "inputs":
			switch inputs := mapItem.Value.V.(type) {
			case Map[any]:
				e.Inputs = Attr(inputs, mapItem.Key.Line, mapItem.Key.Column, mapItem.Key.HeadComment, mapItem.Value.LineComment)
			default:
				return newAttrErrorf(mapItem.Value, "inputs must be a map")
			}
		default:
			return newAttrErrorf(mapItem.Key, "unexpected attribute: %s. must be one of [source, inputs]", mapItem.Key.V)
		}
	}
	return nil
}
