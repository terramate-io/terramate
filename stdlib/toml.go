// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib

import (
	"bytes"
	stdjson "encoding/json"
	errstd "errors"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/json"
)

// TomlExperimentName is the name for the TOML experiment.
const TomlExperimentName = "toml-functions"

// ErrTomlDecode represents errors happening during decoding of TOML content.
const ErrTomlDecode errors.Kind = "failed to decode toml content"

// TomlEncode implements the `tm_tomlencode()` function.
func TomlEncode() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "val",
				Type: cty.DynamicPseudoType,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return tomlEncode(args[0])
		},
	})
}

// TomlDecode implements the `tm_tomldecode` function.
func TomlDecode() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "content",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return tomlDecode(args[0].AsString())
		},
	})
}

func tomlEncode(val cty.Value) (cty.Value, error) {
	data, err := json.Marshal(val, cty.DynamicPseudoType)
	if err != nil {
		return cty.NilVal, err
	}
	d := stdjson.NewDecoder(bytes.NewBuffer(data))
	var out bytes.Buffer
	e := toml.NewEncoder(&out)
	var v map[string]interface{}
	err = d.Decode(&v)
	if err != nil {
		return cty.NilVal, err
	}
	err = e.Encode(v["value"])
	if err != nil {
		return cty.NilVal, err
	}
	return cty.StringVal(out.String()), nil
}

func tomlDecode(content string) (cty.Value, error) {
	var v interface{}

	d := toml.NewDecoder(strings.NewReader(content))
	err := d.Decode(&v)
	if err != nil {
		var derr *toml.DecodeError
		if errstd.As(err, &derr) {
			row, col := derr.Position()
			return cty.NilVal, errors.E("%s\nerror occurred at row %d column %d", derr.String(), row, col)
		}
		return cty.NilVal, err
	}

	var jsonOut bytes.Buffer
	e := stdjson.NewEncoder(&jsonOut)

	err = e.Encode(v)
	if err != nil {
		return cty.NilVal, errors.E(ErrTomlDecode, err)
	}

	jsonBytes := jsonOut.Bytes()
	t, err := json.ImpliedType(jsonBytes)
	if err != nil {
		return cty.NilVal, errors.E(ErrTomlDecode, "cannot determine an HCL type for decoded toml content")
	}
	val, err := json.Unmarshal(jsonBytes, t)
	if err != nil {
		return cty.NilVal, errors.E(ErrTomlDecode, "unmarshaling from json: %s", err.Error())
	}
	return val, nil
}
