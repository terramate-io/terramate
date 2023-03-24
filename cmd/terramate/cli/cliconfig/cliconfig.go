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
	"github.com/zclconf/go-cty/cty"
)

// Config is the evaluated CLI configuration options.
type Config struct {
	DisableCheckpoint          bool
	DisableCheckpointSignature bool
}

// LoadAll loads (parses and evaluates) all CLI configuration files.
func LoadAll() (cfg Config, err error) {
	abspath, found := configAbsPath()
	if !found {
		return cfg, nil
	}

	content, err := os.ReadFile(abspath)
	if err != nil {
		return cfg, nil
	}

	parser := hclparse.NewParser()
	hclfile, diags := parser.ParseHCL(content, abspath)
	if diags.HasErrors() {
		return cfg, errors.E(diags, "failed to parse %s", abspath)
	}

	body := hclfile.Body.(*hclsyntax.Body)
	disableCheckpointAttr, ok := body.Attributes["disable_checkpoint"]
	if ok {
		val, diags := disableCheckpointAttr.Expr.Value(nil)
		if diags.HasErrors() {
			return cfg, errors.E(diags, `failed to evaluate the "disable_checkpoint" attribute`)
		}
		if !val.Type().Equals(cty.Bool) {
			return cfg, errors.E(
				`the "disable_checkpoint" attribute expects a boolean value but type %s was given (value %s)`,
				val.Type().FriendlyName(), hclwrite.TokensForValue(val),
			)
		}

		cfg.DisableCheckpoint = val.True()
	}

	disableCheckpointSignatureAttr, ok := body.Attributes["disable_checkpoint_signature"]
	if ok {
		val, diags := disableCheckpointSignatureAttr.Expr.Value(nil)
		if diags.HasErrors() {
			return cfg, errors.E(diags, `failed to evaluate the "disable_checkpoint_signature" attribute`)
		}
		if !val.Type().Equals(cty.Bool) {
			return cfg, errors.E(
				`the "disable_checkpoint_signature" attribute expects a boolean value but type %s was given (value %s)`,
				val.Type().FriendlyName(), hclwrite.TokensForValue(val),
			)
		}
		cfg.DisableCheckpointSignature = val.True()
	}
	return cfg, nil
}
