// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tf

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

// ErrHCLSyntax represents a HCL syntax error
const ErrHCLSyntax errors.Kind = "HCL syntax error"

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return (len(m.Source) >= 2 && m.Source[0:2] == "./") ||
		(len(m.Source) >= 3 && m.Source[0:3] == "../")
}

// ParseModules parses blocks of type "module" containing a single label.
func ParseModules(path string) ([]Module, error) {
	logger := log.With().
		Str("action", "ParseModules()").
		Str("path", path).
		Logger()

	logger.Trace().Msg("Get path information.")

	_, err := os.Stat(path)
	if err != nil {
		return nil, errors.E(err, "stat failed on %q", path)
	}

	logger.Trace().Msg("Create new parser")

	p := hclparse.NewParser()

	logger.Debug().Msg("Parse HCL file")

	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errors.E(ErrHCLSyntax, diags)
	}

	body := f.Body.(*hclsyntax.Body)

	logger.Trace().Msg("Parse modules")

	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}

		var moduleName string

		if len(block.Labels) == 1 {
			moduleName = block.Labels[0]
		} else {
			logger.Debug().Msgf("ignoring module block with %d labels", len(block.Labels))

			continue
		}
		logger = logger.With().
			Str("module", moduleName).
			Logger()

		logger.Trace().Msg("Get source attribute.")
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			logger.Debug().
				Err(err).
				Msg("ignoring module block without source")

			continue
		}
		if !ok {
			logger.Debug().Msg("ignoring module block without source")

			continue
		}
		modules = append(modules, Module{Source: source})
	}

	return modules, nil
}

// IsStack tells if the file defined by path is a potential stack.
// Eg.: has a backend block or a provider block.
func IsStack(path string) (bool, error) {
	logger := log.With().
		Str("action", "IsStack").
		Logger()

	p := hclparse.NewParser()

	logger.Debug().Msg("Parsing TF file")

	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return false, errors.E(ErrHCLSyntax, diags)
	}

	body := f.Body.(*hclsyntax.Body)

	logger.Trace().Msg("Parse terraform.backend blocks")

	for _, block := range body.Blocks {
		switch block.Type {
		case "terraform":
			for _, block := range block.Body.Blocks {
				if block.Type != "backend" {
					continue
				}

				logger.Trace().Msg("found a backend block")
				return true, nil
			}
		case "provider":
			return true, nil
		}
	}

	return false, nil
}

func findStringAttr(block *hclsyntax.Block, attrName string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().Msg("Range over attributes.")

	attrs := ast.AsHCLAttributes(block.Body.Attributes)

	for _, attr := range ast.SortRawAttributes(attrs) {
		if attrName != attr.Name {
			continue
		}

		logger.Trace().Msg("Found attribute that we were looking for.")
		logger.Trace().Msg("Get attribute value.")
		attrVal, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return "", false, errors.E(diags)
		}

		logger.Trace().Msg("Check value type is correct.")
		if attrVal.Type() != cty.String {
			return "", false, errors.E(
				"attribute %q is not a string", attr.Name, attr.Expr.Range(),
			)
		}

		return attrVal.AsString(), true, nil
	}

	return "", false, nil
}
