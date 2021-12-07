package hcl

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terrastack are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

type Terrastack struct {
	// RequiredVersion contains the terrastack version required by the stack.
	RequiredVersion string
}

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
func (p *Parser) ParseModules(path string) ([]Module, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	f, diags := p.p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing modules: %w", diags)
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
			return nil, fmt.Errorf("looking for %q.source attribute: %w",
				moduleName, err)
		}
		if !ok {
			continue
		}

		modules = append(modules, Module{Source: source})
	}

	return modules, nil
}

// Parse parses a terrastack source.
func (p *Parser) Parse(fname string, data []byte) (*Terrastack, error) {
	f, diags := p.p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing terrastack: %w", diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var terrastack []*Terrastack
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

		terrastack = append(terrastack, &Terrastack{
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
func (p *Parser) ParseFile(path string) (*Terrastack, error) {
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
