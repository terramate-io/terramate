package hhcl

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/zclconf/go-cty/cty"
)

var SyntaxError error = fmt.Errorf("syntax error")

// Parser is a High-Level parser.
type Parser struct {
	p *hclparse.Parser
}

// NewParser creates a HCL parser
func NewParser() *Parser {
	return &Parser{
		p: hclparse.NewParser(),
	}
}

// ParseModules parses blocks of type "module" containing a single label.
func (p *Parser) ParseModules(path string) ([]hcl.Module, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	f, diags := p.p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing modules: %w", diags)
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		panic("internal error: *hcl.File.Body is not a *hclsyntax.Body")
	}

	var modules []hcl.Module

outer:
	for _, block := range body.Blocks {
		if block.Type == "module" {
			if len(block.Labels) != 1 {
				// a module block must have 1 label.
				continue
			}

			module := block.Labels[0]
			for name, value := range block.Body.Attributes {
				if name == "source" {
					sourceVal, diags := value.Expr.Value(nil)
					if diags.HasErrors() {
						return nil, fmt.Errorf("failed to evaluate %q.source attribute: %w",
							module, diags)
					}
					if sourceVal.Type() != cty.String {
						return nil, fmt.Errorf("%q.source is not a string", module)
					}
					modules = append(modules, hcl.Module{Source: sourceVal.AsString()})
					continue outer
				}
			}
		}
	}

	return modules, nil
}
