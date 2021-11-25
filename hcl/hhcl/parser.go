package hhcl

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/zclconf/go-cty/cty"
)

// Parser is a terrastack parser.
type TSParser struct {
	p *hclparse.Parser
}

// NewParser creates a HCL parser
func NewTSParser() *TSParser {
	return &TSParser{
		p: hclparse.NewParser(),
	}
}

// ParseModules parses blocks of type "module" containing a single label.
func (p *TSParser) ParseModules(path string) ([]hcl.Module, error) {
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
		source, ok, err := findStringAttr(block, "source")
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

// Parse parses a terrastack source.
func (p *TSParser) Parse(fname string, data []byte) (*hcl.Terrastack, error) {
	f, diags := p.p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing terrastack: %w", diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var terrastack []*hcl.Terrastack
	for _, block := range body.Blocks {
		if block.Type != "terrastack" {
			continue
		}

		if len(block.Labels) > 0 {
			return nil, fmt.Errorf("terrastack block must have no labels")
		}

		reqversion, ok, err := findStringAttr(block, "required_version")
		if err != nil {
			return nil, fmt.Errorf("looking for terrastack.required_version attribute: %w",
				err)
		}
		if !ok {
			return nil, fmt.Errorf("terrastack block requires a \"required_version\" attribute")
		}

		terrastack = append(terrastack, &hcl.Terrastack{
			RequiredVersion: reqversion,
		})
	}

	if len(terrastack) > 1 {
		return nil, fmt.Errorf("only 1 \"terrastack\" block is allowed but found %d",
			len(terrastack))
	}

	if len(terrastack) == 0 {
		return nil, fmt.Errorf("no \"terrastack\" block found")
	}

	return terrastack[0], nil
}

// ParseFile parses a terrastack file.
func (p *TSParser) ParseFile(path string) (*hcl.Terrastack, error) {
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
