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
	"errors"
	"fmt"
	"os"
	"sort"

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

type Config struct {
	Terramate *Terramate
	Stack     *Stack
}

// Terramate is the parsed "terramate" HCL block.
type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	Backend *hclsyntax.Block
}

// Stack is the parsed "stack" HCL block.
type Stack struct {
	// Name of the stack
	Name string

	// After is a list of non-duplicated stack entries that must run after the
	// current stack runs.
	After []string
}

const (
	ErrHCLSyntax                errutil.Error = "HCL syntax error"
	ErrNoTerramateBlock         errutil.Error = "no \"terramate\" block found"
	ErrMalformedTerramateConfig errutil.Error = "malformed terramate config"
	ErrMalformedTerraform       errutil.Error = "malformed terraform"
	ErrStackInvalidRunOrder     errutil.Error = "invalid stack execution order definition"
)

// ParseModules parses blocks of type "module" containing a single label.
func ParseModules(path string) ([]Module, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	p := hclparse.NewParser()
	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing modules: %w", diags),
		)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}

		if len(block.Labels) != 1 {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				fmt.Errorf("module block must have 1 label"),
			)
		}

		moduleName := block.Labels[0]
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				fmt.Errorf("looking for module.%q.source attribute: %w",
					moduleName, err),
			)
		}
		if !ok {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				errors.New("module must have a \"source\" attribute"),
			)
		}

		modules = append(modules, Module{Source: source})
	}

	return modules, nil
}

// ParseBody parses HCL and return the parsed body.
func ParseBody(src []byte, filename string) (*hclsyntax.Body, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCL(src, filename)
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
func Parse(fname string, data []byte) (*Config, error) {
	p := hclparse.NewParser()
	f, diags := p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return nil, errutil.Chain(ErrHCLSyntax, diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	var tmconfig Config
	var tmblock, stackblock *hclsyntax.Block
	var foundtm, foundstack bool
	for _, block := range body.Blocks {
		if block.Type != "terramate" && block.Type != "stack" {
			return nil, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("block type %q is not supported", block.Type),
			)
		}

		if block.Type == "terramate" {
			if foundtm {
				return nil, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple terramate blocks in file %q", fname),
				)
			}
			foundtm = true
			tmblock = block
			continue
		}

		if block.Type == "stack" {
			if foundstack {
				return nil, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple stack blocks in file %q", fname),
				)
			}

			foundstack = true
			stackblock = block
		}
	}

	if !foundtm {
		return nil, ErrNoTerramateBlock
	}

	if len(tmblock.Labels) > 0 {
		return nil, errutil.Chain(
			ErrMalformedTerramateConfig,
			fmt.Errorf("terramate block must have no labels"),
		)
	}

	tmconfig.Terramate = &Terramate{}
	tm := tmconfig.Terramate

	for name, value := range tmblock.Body.Attributes {
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
					ErrMalformedTerramateConfig,
					fmt.Errorf("attribute %q is not a string", name),
				)
			}

			tm.RequiredVersion = attrVal.AsString()

		default:
			return nil, errutil.Chain(ErrMalformedTerramateConfig,
				fmt.Errorf("invalid attribute %q", name),
			)
		}
	}

	foundBackend := false
	for _, block := range tmblock.Body.Blocks {
		if block.Type != "backend" {
			return nil, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("block type %q not supported", block.Type))
		}

		if foundBackend {
			return nil, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("multiple backend blocks in file %q", fname),
			)
		}

		foundBackend = true

		if len(block.Labels) != 1 {
			return nil, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("backend type expects 1 label but given %v",
					block.Labels),
			)
		}

		tm.Backend = block
	}

	if !foundstack {
		return &tmconfig, nil
	}

	tmconfig.Stack = &Stack{}
	stack := tmconfig.Stack

	for name, value := range stackblock.Body.Attributes {
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, errutil.Chain(
				ErrHCLSyntax,
				fmt.Errorf("failed to evaluate %q attribute: %w",
					name, diags),
			)
		}
		switch name {
		case "name":
			if attrVal.Type() != cty.String {
				return nil, errutil.Chain(ErrMalformedTerramateConfig,
					fmt.Errorf("field stack.\"name\" must be a \"string\" but given %q",
						attrVal.Type().FriendlyName()),
				)
			}
			stack.Name = attrVal.AsString()
		case "after":
			err := assignSet(name, &stack.After, attrVal)
			if err != nil {
				return nil, err
			}
		}
	}

	return &tmconfig, nil
}

// ParseFile parses a terramate file.
func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}

	return Parse(path, data)
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
			return errutil.Chain(ErrStackInvalidRunOrder,
				fmt.Errorf("field %q is a set(string) but contains %q",
					name, elem.Type().FriendlyName()),
			)
		}

		str := elem.AsString()
		if _, ok := values[str]; ok {
			return errutil.Chain(ErrStackInvalidRunOrder,
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

	sort.Strings(elems)
	*target = elems
	return nil
}

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}
