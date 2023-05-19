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

package cliconfig

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/zclconf/go-cty/cty"
)

const configPathEnv = "TM_CLI_CONFIG_FILE"

const (
	// ErrInvalidAttributeType indicates the attribute has an invalid type.
	ErrInvalidAttributeType errors.Kind = "attribute with invalid type"

	// ErrUnrecognizedAttribute indicates the attribute is unrecognized.
	ErrUnrecognizedAttribute errors.Kind = "unrecognized attribute"
)

// Config is the evaluated CLI configuration options.
type Config struct {
	DisableCheckpoint          bool
	DisableCheckpointSignature bool
	UserTerramateDir           string
}

// Load loads (parses and evaluates) all CLI configuration files.
func Load() (cfg Config, err error) {
	fname := os.Getenv(configPathEnv)
	if fname == "" {
		var found bool
		fname, found = configAbsPath()
		if !found {
			return cfg, nil
		}
	}
	return LoadFrom(fname)
}

// LoadFrom loads the CLI configuration file from fname.
func LoadFrom(fname string) (Config, error) {
	content, err := os.ReadFile(fname)
	if err != nil {
		return Config{}, nil
	}

	parser := hclparse.NewParser()
	hclfile, diags := parser.ParseHCL(content, fname)
	if diags.HasErrors() {
		return Config{}, errors.E(hcl.ErrHCLSyntax, diags, "failed to parse %s", fname)
	}

	var cfg Config
	body := hclfile.Body.(*hclsyntax.Body)
	for name, attr := range body.Attributes {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return Config{}, errors.E(diags, eval.ErrEval, `failed to evaluate the "%s" attribute`, name)
		}
		switch name {
		case "disable_checkpoint":
			if err := checkBoolType(val, name); err != nil {
				return Config{}, err
			}
			cfg.DisableCheckpoint = val.True()
		case "disable_checkpoint_signature":
			if err := checkBoolType(val, name); err != nil {
				return Config{}, err
			}
			cfg.DisableCheckpointSignature = val.True()
		case "user_terramate_dir":
			if err := checkStrType(val, name); err != nil {
				return Config{}, err
			}
			cfg.UserTerramateDir = val.AsString()
		default:
			return cfg, errors.E(ErrUnrecognizedAttribute, name)
		}
	}

	return cfg, nil
}

func checkBoolType(val cty.Value, name string) error {
	if !val.Type().Equals(cty.Bool) {
		return errors.E(
			ErrInvalidAttributeType,
			`%q attribute expects a boolean value but a value of type %s was given (value %s)`,
			name, val.Type().FriendlyName(), hclwrite.TokensForValue(val),
		)
	}
	return nil
}

func checkStrType(val cty.Value, name string) error {
	if !val.Type().Equals(cty.String) {
		return errors.E(
			ErrInvalidAttributeType,
			`%q attribute expects an string value but a value of type %s was given (value %s)`,
			name, val.Type().FriendlyName(), hclwrite.TokensForValue(val),
		)
	}
	return nil
}
