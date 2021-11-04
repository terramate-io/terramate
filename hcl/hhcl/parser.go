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

	body, _ := f.Body.(*hclsyntax.Body)

	var modules []hcl.Module
	for _, block := range body.Blocks {
		if block.Type != "module" || len(block.Labels) != 1 {
			continue
		}

		moduleName := block.Labels[0]
		source, err, ok := findSourceAttribute(block)
		if err != nil {
			return nil, fmt.Errorf("looking for %q.source attribute: %w",
				moduleName, err)
		}
		if !ok {
			continue
		}

		modules = append(modules, hcl.Module{Source: source})
	}

	return modules, nil
}

func findSourceAttribute(block *hclsyntax.Block) (string, error, bool) {
	for name, value := range block.Body.Attributes {
		if name != "source" {
			continue
		}

		sourceVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return "", fmt.Errorf("failed to evaluate source attribute: %w",
				diags), false
		}

		if sourceVal.Type() != cty.String {
			continue
		}

		return sourceVal.AsString(), nil, true
	}

	return "", nil, false
}
