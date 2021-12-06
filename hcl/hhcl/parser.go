package hhcl

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terrastack/hcl"
	"github.com/zclconf/go-cty/cty"
)

// Parser is a terrastack parser.
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
func (p *Parser) Parse(fname string, data []byte) (*hcl.Terrastack, error) {
	f, diags := p.p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return nil, errutil.Chain(hcl.ErrHCLSyntax, diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var tsblock hcl.Terrastack
	var found bool
	for _, block := range body.Blocks {
		if block.Type != "terrastack" {
			continue
		}

		if found {
			return nil, fmt.Errorf("multiple terrastack blocks in file %q", fname)
		}

		found = true

		if len(block.Labels) > 0 {
			return nil, fmt.Errorf("terrastack block must have no labels")
		}

		for name, value := range block.Body.Attributes {
			attrVal, diags := value.Expr.Value(nil)
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to evaluate %q attribute: %w",
					name, diags)
			}
			switch name {
			case "required_version":
				if attrVal.Type() != cty.String {
					return nil, fmt.Errorf("attribute %q is not a string", name)
				}

				tsblock.RequiredVersion = attrVal.AsString()
			case "after":
				err := assignSet(name, &tsblock.After, attrVal)
				if err != nil {
					return nil, err
				}
			case "before":
				err := assignSet(name, &tsblock.Before, attrVal)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if !found {
		return nil, hcl.ErrNoTerrastackBlock
	}

	return &tsblock, nil
}

// ParseFile parses a terrastack file.
func (p *Parser) ParseFile(path string) (*hcl.Terrastack, error) {
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

func assignSet(name string, target *[]string, val cty.Value) error {
	if val.Type().IsSetType() {
		return fmt.Errorf("attribute %q is not a set", name)
	}

	values := map[string]struct{}{}
	iterator := val.ElementIterator()
	for iterator.Next() {
		_, elem := iterator.Element()
		if elem.Type() != cty.String {
			return errutil.Chain(hcl.ErrInvalidRunOrder,
				fmt.Errorf("field %q is a set(string) but contains %q",
					name, elem.Type().FriendlyName()),
			)
		}

		str := elem.AsString()
		if _, ok := values[str]; ok {
			return errutil.Chain(hcl.ErrInvalidRunOrder,
				fmt.Errorf("duplicated entry %q in field %q of type set(string)",
					str, name),
			)
		}
		values[str] = struct{}{}
	}

	var elems []string
	for v := range values {
		elems = append(elems, v)
	}
	*target = elems
	return nil
}
