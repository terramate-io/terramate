// Copyright 2021 Mineiros GmbH
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

package hcl

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/errutil"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	Backend *hclsyntax.Block
}

// Parser is a terramate parser.
type Parser struct {
	p *hclparse.Parser
}

const (
	ErrHCLSyntax               errutil.Error = "HCL syntax error"
	ErrNoTerramateBlock        errutil.Error = "no \"terramate\" block found"
	ErrMalformedTerramateBlock errutil.Error = "malformed terramate block"
	ErrMalformedTerraform      errutil.Error = "malformed terraform"
)

// NewParser creates a HCL parser
func NewParser() *Parser {
	return &Parser{
		p: hclparse.NewParser(),
	}
}

// ParseModules parses blocks of type "module" containing a single label.
func (p *Parser) ParseModules(path string) ([]Module, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	f, diags := p.p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing modules: %w", diags),
		)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" || len(block.Labels) != 1 {
			continue
		}

		moduleName := block.Labels[0]
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				fmt.Errorf("looking for %q.source attribute: %w",
					moduleName, err),
			)
		}
		if !ok {
			continue
		}

		modules = append(modules, Module{Source: source})
	}

	return modules, nil
}

// ParseBody parses HCL and return the parsed body.
func (p *Parser) ParseBody(src []byte, filename string) (*hclsyntax.Body, error) {
	f, diags := p.p.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing modules: %w", diags),
		)
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("expected to parse body, got[%v] type[%[1]T]", f.Body)
	}
	return body, nil
}

// Parse parses a terramate source.
func (p *Parser) Parse(fname string, data []byte) (*Terramate, error) {
	f, diags := p.p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return nil, errutil.Chain(ErrHCLSyntax, diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var tsconfig Terramate
	var tsblock *hclsyntax.Block
	var found bool
	for _, block := range body.Blocks {
		if block.Type != "terramate" {
			continue
		}

		if found {
			return nil, errutil.Chain(
				ErrMalformedTerramateBlock,
				fmt.Errorf("multiple terramate blocks in file %q", fname),
			)
		}

		found = true
		tsblock = block
	}

	if !found {
		return nil, ErrNoTerramateBlock
	}

	if len(tsblock.Labels) > 0 {
		return nil, errutil.Chain(
			ErrMalformedTerramateBlock,
			fmt.Errorf("terramate block must have no labels"),
		)
	}

	for name, value := range tsblock.Body.Attributes {
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, errutil.Chain(
				ErrHCLSyntax,
				fmt.Errorf("failed to evaluate %q attribute: %w",
					name, diags),
			)
		}
		switch name {
		case "required_version":
			if attrVal.Type() != cty.String {
				return nil, errutil.Chain(
					ErrMalformedTerramateBlock,
					fmt.Errorf("attribute %q is not a string", name),
				)
			}

			tsconfig.RequiredVersion = attrVal.AsString()

			// TODO(i4k): support other fields in the case (after, before, etc)
		default:
			return nil, errutil.Chain(ErrMalformedTerramateBlock,
				fmt.Errorf("invalid attribute %q", name),
			)
		}
	}

	found = false
	for _, block := range tsblock.Body.Blocks {
		if block.Type != "backend" {
			return nil, errutil.Chain(
				ErrMalformedTerramateBlock,
				fmt.Errorf("block type %q not supported", block.Type))
		}

		if found {
			return nil, errutil.Chain(
				ErrMalformedTerramateBlock,
				fmt.Errorf("multiple backend blocks in file %q", fname),
			)
		}

		found = true

		if len(block.Labels) != 1 {
			return nil, errutil.Chain(
				ErrMalformedTerramateBlock,
				fmt.Errorf("backend type expects 1 label but given %v",
					block.Labels),
			)
		}

		tsconfig.Backend = block
	}

	return &tsconfig, nil
}

// ParseFile parses a terramate file.
func (p *Parser) ParseFile(path string) (*Terramate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}

	return p.Parse(path, data)
}

func findStringAttr(block *hclsyntax.Block, attr string) (string, bool, error) {
	for name, value := range block.Body.Attributes {
		if name != attr {
			continue
		}

		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return "", false, fmt.Errorf("failed to evaluate %q attribute: %w",
				attr, diags)
		}

		if attrVal.Type() != cty.String {
			return "", false, fmt.Errorf("attribute %q is not a string", attr)
		}

		return attrVal.AsString(), true, nil
	}

	return "", false, nil
}

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}
