// Copyright 2022 Mineiros GmbH
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

package tf

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

// Errors returned during the terraform parsing.
const (
	ErrHCLSyntax       errors.Kind = "HCL syntax error"
	ErrTerraformSchema errors.Kind = "terraform schema error"
)

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
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

	errs := errors.L()
	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}

		var moduleName string

		if len(block.Labels) == 1 {
			moduleName = block.Labels[0]
		} else {
			errs.Append(errors.E(ErrTerraformSchema, block.OpenBraceRange,
				"\"module\" block must have 1 label"))
		}

		logger.Trace().Msg("Get source attribute.")
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			errs.Append(errors.E(ErrTerraformSchema, err,
				"looking for module.%q.source attribute", moduleName))
		}
		if !ok {
			errs.Append(errors.E(ErrTerraformSchema,
				hcl.RangeBetween(block.OpenBraceRange, block.CloseBraceRange),
				"module must have a \"source\" attribute",
			))
		}
		modules = append(modules, Module{Source: source})
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return modules, nil
}

func findStringAttr(block *hclsyntax.Block, attrName string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().Msg("Range over attributes.")
	for _, attr := range ast.SortRawAttributes(block.Body.Attributes) {
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
