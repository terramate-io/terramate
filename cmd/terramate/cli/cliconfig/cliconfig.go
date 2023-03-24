package cliconfig

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/zclconf/go-cty/cty"
)

type Config struct {
	DisableCheckpoint          bool
	DisableCheckpointSignature bool
}

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
