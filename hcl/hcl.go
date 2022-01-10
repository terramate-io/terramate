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
	"github.com/rs/zerolog/log"
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

type GitConfig struct {
	BaseRef              string // BaseRef is the general base git ref.
	DefaultBranchBaseRef string // DefaultBranchBaseRef is the baseRef when in default branch.
	DefaultBranch        string // DefaultBranch is the default branch.
	DefaultRemote        string // DefaultRemote is the default remote.
}

type RootConfig struct {
	Git GitConfig
}

// Terramate is the parsed "terramate" HCL block.
type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	// RootConfig is the configuration at the project root directory (commonly
	// the git directory).
	RootConfig *RootConfig

	Backend *hclsyntax.Block
}

// Stack is the parsed "stack" HCL block.
type Stack struct {
	// Name of the stack
	Name string

	// After is a list of non-duplicated stack entries that must run after the
	// current stack runs.
	After []string

	// Before is a list of non-duplicated stack entries that must run before the
	// current stack runs.
	Before []string
}

const (
	ErrHCLSyntax                errutil.Error = "HCL syntax error"
	ErrMalformedTerramateConfig errutil.Error = "malformed terramate config"
	ErrMalformedTerraform       errutil.Error = "malformed terraform"
	ErrStackInvalidRunOrder     errutil.Error = "invalid stack execution order definition"
)

func NewConfig(reqversion string) Config {
	return Config{
		Terramate: &Terramate{
			RequiredVersion: reqversion,
		},
	}
}

// ParseModules parses blocks of type "module" containing a single label.
func ParseModules(path string) ([]Module, error) {
	logger := log.With().
		Str("action", "ParseModules()").
		Str("path", path).
		Logger()

	logger.Trace().
		Msg("Get path information.")
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	logger.Trace().
		Msg("Create new parser.")
	p := hclparse.NewParser()

	logger.Debug().
		Msg("Parse HCL file.")
	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing modules: %w", diags),
		)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	logger.Trace().
		Msg("Parse modules.")
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

		logger.Trace().
			Msg("Get source attribute.")
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
	logger := log.With().
		Str("action", "ParseBody()").
		Logger()

	logger.Trace().
		Str("path", filename).
		Msg("Parse file.")
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
func Parse(fname string, data []byte) (Config, error) {
	logger := log.With().
		Str("action", "Parse()").
		Str("path", fname).
		Logger()

	logger.Debug().
		Msg("Parse file.")
	p := hclparse.NewParser()
	f, diags := p.ParseHCL(data, fname)
	if diags.HasErrors() {
		return Config{}, errutil.Chain(ErrHCLSyntax, diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	logger.Trace().
		Msg("Range over attributes.")
	for name := range body.Attributes {
		return Config{}, errutil.Chain(
			ErrMalformedTerramateConfig,
			fmt.Errorf("unrecognized attribute %q", name),
		)
	}

	var tmconfig Config
	var tmblock, stackblock *hclsyntax.Block
	var foundtm, foundstack bool

	logger.Trace().
		Msg("Range over blocks.")
	for _, block := range body.Blocks {
		if !blockIsAllowed(block.Type) {
			return Config{}, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("block type %q is not supported", block.Type),
			)
		}

		if block.Type == "terramate" {
			logger.Trace().
				Msg("Found 'terramate' block type.")
			if foundtm {
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple terramate blocks in file %q", fname),
				)
			}
			foundtm = true
			tmblock = block
			continue
		}

		if block.Type == "stack" {
			logger.Trace().
				Msg("Found stack block type.")
			if foundstack {
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple stack blocks in file %q", fname),
				)
			}

			foundstack = true
			stackblock = block
		}
	}

	if foundtm {
		logger.Trace().
			Msg("Found terramate block type.")
		if len(tmblock.Labels) > 0 {
			return Config{}, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("terramate block must have no labels"),
			)
		}

		tmconfig.Terramate = &Terramate{}
		tm := tmconfig.Terramate

		logger.Trace().
			Msg("Range over terramate block attributes.")
		for name, value := range tmblock.Body.Attributes {
			attrVal, diags := value.Expr.Value(nil)
			if diags.HasErrors() {
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("failed to evaluate %q attribute: %w",
						name, diags),
				)
			}
			switch name {
			case "required_version":
				logger.Trace().
					Msg("Parsing  attribute 'required_version'.")
				if attrVal.Type() != cty.String {
					return Config{}, errutil.Chain(
						ErrMalformedTerramateConfig,
						fmt.Errorf("attribute %q is not a string", name),
					)
				}

				tm.RequiredVersion = attrVal.AsString()

			default:
				return Config{}, errutil.Chain(ErrMalformedTerramateConfig,
					fmt.Errorf("invalid attribute %q", name),
				)
			}
		}

		foundBackend := false
		foundConfig := false

		logger.Trace().
			Msg("Range over terramate blocks")
		for _, block := range tmblock.Body.Blocks {
			switch block.Type {
			case "backend":
				logger.Trace().
					Msg("Parsing backend block.")
				if foundBackend {
					return Config{}, errutil.Chain(
						ErrMalformedTerramateConfig,
						fmt.Errorf("multiple backend blocks in file %q", fname),
					)
				}

				if len(block.Labels) != 1 {
					return Config{}, errutil.Chain(
						ErrMalformedTerramateConfig,
						fmt.Errorf("backend type expects 1 label but given %v",
							block.Labels),
					)
				}

				foundBackend = true
				tm.Backend = block

			case "config":
				logger.Trace().
					Msg("Found config block.")
				if foundConfig {
					return Config{}, errutil.Chain(
						ErrMalformedTerramateConfig,
						fmt.Errorf("multiple config blocks in file %q", fname),
					)
				}

				logger.Trace().
					Msg("Parse root config.")
				rootConfig := RootConfig{}
				tm.RootConfig = &rootConfig
				err := parseRootConfig(&rootConfig, block)
				if err != nil {
					return Config{}, err
				}

				foundConfig = true
			default:
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("block type %q not supported", block.Type))
			}

		}
	}

	if !foundstack {
		return tmconfig, nil
	}

	logger.Debug().
		Msg("Parse stack.")
	tmconfig.Stack = &Stack{}
	err := parseStack(tmconfig.Stack, stackblock)
	if err != nil {
		return Config{}, err
	}

	return tmconfig, nil
}

// ParseFile parses a terramate file.
func ParseFile(path string) (Config, error) {
	logger := log.With().
		Str("action", "ParseFile()").
		Str("path", path).
		Logger()

	logger.Debug().
		Msg("Read file.")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read file %q: %w", path, err)
	}

	return Parse(path, data)
}

// ParseGlobalsBlocks parses globals blocks, ignoring any other blocks
func ParseGlobalsBlocks(path string) ([]*hclsyntax.Block, error) {
	return parseBlocksOfType(path, "globals")
}

// ParseExportAsLocalsBlocks parses export_as_locals blocks, ignoring other blocks
func ParseExportAsLocalsBlocks(path string) ([]*hclsyntax.Block, error) {
	return parseBlocksOfType(path, "export_as_locals")
}

func parseBlocksOfType(path string, blocktype string) ([]*hclsyntax.Block, error) {
	logger := log.With().
		Str("action", "parseBlocksOfType()").
		Str("path", path).
		Logger()

	logger.Trace().
		Msg("Get file info.")
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	logger.Debug().
		Msg("Parse file.")
	p := hclparse.NewParser()
	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing blocks of type %q: %w", blocktype, diags),
		)
	}

	body, _ := f.Body.(*hclsyntax.Body)
	return filterBlocksByType(blocktype, body.Blocks), nil
}

func findStringAttr(block *hclsyntax.Block, attr string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().
		Msg("Range over attributes.")
	for name, value := range block.Body.Attributes {
		if name != attr {
			continue
		}

		logger.Trace().
			Msg("Found attribute that we were looking for.")

		logger.Trace().
			Msg("Get attribute value.")
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return "", false, fmt.Errorf("failed to evaluate %q attribute: %w",
				attr, diags)
		}

		logger.Trace().
			Msg("Check value type is correct.")
		if attrVal.Type() != cty.String {
			return "", false, fmt.Errorf("attribute %q is not a string", attr)
		}

		return attrVal.AsString(), true, nil
	}

	return "", false, nil
}

func assignSet(name string, target *[]string, val cty.Value) error {
	logger := log.With().
		Str("action", "assignSet()").
		Logger()

	logger.Trace().
		Msg("Check val is correct type.")
	if val.Type().IsSetType() {
		return fmt.Errorf("attribute %q is not a set", name)
	}

	logger.Trace().
		Msg("Iterate over values.")
	values := map[string]struct{}{}
	iterator := val.ElementIterator()
	for iterator.Next() {
		_, elem := iterator.Element()

		logger.Trace().
			Msg("Check element is of correct type.")
		if elem.Type() != cty.String {
			return errutil.Chain(ErrStackInvalidRunOrder,
				fmt.Errorf("field %q is a set(string) but contains %q",
					name, elem.Type().FriendlyName()),
			)
		}

		logger.Trace().
			Msg("Get element as string.")
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

	logger.Trace().
		Msg("Sort elements.")
	sort.Strings(elems)
	*target = elems
	return nil
}

func parseStack(stack *Stack, stackblock *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseStack()").
		Str("stack", stack.Name).
		Logger()

	logger.Debug().
		Msg("Get stack attributes.")
	for name, value := range stackblock.Body.Attributes {
		logger.Trace().
			Msg("Get attribute value.")
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("failed to evaluate %q attribute: %w",
					name, diags),
			)
		}
		switch name {
		case "name":
			logger.Trace().
				Msg("Attribute name was 'name'.")
			if attrVal.Type() != cty.String {
				return errutil.Chain(ErrMalformedTerramateConfig,
					fmt.Errorf("field stack.\"name\" must be a \"string\" but given %q",
						attrVal.Type().FriendlyName()),
				)
			}
			stack.Name = attrVal.AsString()
		case "after":
			logger.Trace().
				Msg("Attribute name was 'after'.")
			err := assignSet(name, &stack.After, attrVal)
			if err != nil {
				return err
			}

		case "before":
			logger.Trace().
				Msg("Attribute name was 'before'.")
			err := assignSet(name, &stack.Before, attrVal)
			if err != nil {
				return err
			}

		default:
			return errutil.Chain(ErrMalformedTerramateConfig,
				fmt.Errorf("unrecognized attribute stack.%q", name),
			)
		}
	}

	return nil
}

func parseRootConfig(cfg *RootConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseRootConfig()").
		Logger()

	if len(block.Labels) != 0 {
		return errutil.Chain(
			ErrMalformedTerramateConfig,
			fmt.Errorf("config type expects 0 label but given %v",
				block.Labels),
		)
	}

	logger.Trace().
		Msg("Range over block attributes.")
	for name := range block.Body.Attributes {
		return errutil.Chain(ErrMalformedTerramateConfig,
			fmt.Errorf("unrecognized attribute terramate.config.%s", name),
		)
	}

	foundGit := false

	logger.Trace().
		Msg("Range over blocks.")
	for _, b := range block.Body.Blocks {
		switch b.Type {
		case "git":
			logger.Trace().
				Msg("Type was 'git'.")
			if foundGit {
				return errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple terramate.config.git blocks"),
				)
			}

			foundGit = true
			logger.Trace().
				Msg("Parse git config.")
			err := parseGitConfig(&cfg.Git, b)
			if err != nil {
				return err
			}
		default:
			return errutil.Chain(ErrMalformedTerramateConfig,
				fmt.Errorf("unrecognized block type %q", b.Type),
			)
		}
	}

	return nil
}

func parseGitConfig(git *GitConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().
		Msg("Range over block attributes.")
	for name, value := range block.Body.Attributes {
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return errutil.Chain(
				ErrHCLSyntax,
				fmt.Errorf("failed to evaluate terramate.config.%s attribute: %w",
					name, diags),
			)
		}
		switch name {
		case "default_branch":
			logger.Trace().
				Msg("Attribute name was 'default_branch'.")
			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.branch is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultBranch = attrVal.AsString()
		case "default_remote":
			logger.Trace().
				Msg("Attribute name was 'default_remote'.")
			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.remote is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultRemote = attrVal.AsString()

		case "base_ref":
			logger.Trace().
				Msg("Attribute name was 'base_ref.")
			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.baseRef is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.BaseRef = attrVal.AsString()

		case "default_branch_base_ref":
			logger.Trace().
				Msg("Attribute name was 'default_branch_base_ref.")
			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.defaultBranchBaseRef is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultBranchBaseRef = attrVal.AsString()

		default:
			return errutil.Chain(ErrMalformedTerramateConfig,
				fmt.Errorf("unrecognized attribute terramate.config.git.%s", name),
			)
		}
	}
	return nil
}

func filterBlocksByType(blocktype string, blocks []*hclsyntax.Block) []*hclsyntax.Block {
	logger := log.With().
		Str("action", "filterBlocksByType()").
		Logger()

	var filtered []*hclsyntax.Block

	logger.Trace().
		Msg("Range over blocks.")
	for _, block := range blocks {
		if block.Type != blocktype {
			continue
		}

		filtered = append(filtered, block)
	}

	return filtered
}

func blockIsAllowed(name string) bool {
	logger := log.With().
		Str("action", "blockIsAllowed()").
		Logger()

	switch name {
	case "terramate", "stack", "backend", "globals", "export_as_locals":
		logger.Trace().
			Msg("Block name was allowed.")
		return true
	default:
		return false
	}
}

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}
