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
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

type Config struct {
	// absdir is the absolute path to the configuration directory.
	absdir    string
	Terramate *Terramate
	Stack     *Stack
}

type GitConfig struct {
	BaseRef              string // BaseRef is the general base git ref.
	DefaultBranchBaseRef string // DefaultBranchBaseRef is the baseRef when in default branch.
	DefaultBranch        string // DefaultBranch is the default branch.
	DefaultRemote        string // DefaultRemote is the default remote.
}

// GenerateConfig represents code generation config
type GenerateConfig struct {
	LocalsFilename     string
	BackendCfgFilename string
}

type RootConfig struct {
	Git      *GitConfig
	Generate *GenerateConfig
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

	// Description of the stack
	Description string

	// After is a list of non-duplicated stack entries that must run before the
	// current stack runs.
	After []string

	// Before is a list of non-duplicated stack entries that must run after the
	// current stack runs.
	Before []string

	// Wants is a list of non-duplicated stack entries that must be selected
	// whenever the current stack is selected.
	Wants []string
}

// HCLBlocks maps a filename to a slice of blocks associated with it
type HCLBlocks map[string][]*hclsyntax.Block

const (
	ErrHCLSyntax                errutil.Error = "HCL syntax error"
	ErrMalformedTerramateConfig errutil.Error = "malformed terramate config"
	ErrMalformedTerraform       errutil.Error = "malformed terraform"
	ErrStackInvalidRunOrder     errutil.Error = "invalid stack execution order definition"
)

// NewConfig creates a new HCL config with dir as config directory path.
// If reqversion is provided then it also creates a terramate block with
// RequiredVersion set to this value.
func NewConfig(dir string, reqversion string) (Config, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return Config{}, fmt.Errorf("initializing config: %w", err)
	}

	if !st.IsDir() {
		return Config{}, fmt.Errorf("config constructor requires a directory path")
	}

	var tmblock *Terramate
	if reqversion != "" {
		tmblock = NewTerramate(reqversion)
	}

	return Config{
		Terramate: tmblock,
		absdir:    dir,
	}, nil
}

// AbsDir returns the absolute path of the configuration directory.
func (c Config) AbsDir() string { return c.absdir }

// Save the configuration file using filename inside config directory.
func (c Config) Save(filename string) (err error) {
	cfgpath := filepath.Join(c.absdir, filename)
	f, err := os.Create(cfgpath)
	if err != nil {
		return fmt.Errorf("saving configuration file %q: %w", cfgpath, err)
	}

	defer func() {
		err2 := f.Close()

		if err != nil {
			return
		}

		err = err2
	}()

	return PrintConfig(f, c)
}

// NewTerramate creates a new TerramateBlock with reqversion.
func NewTerramate(reqversion string) *Terramate {
	return &Terramate{
		RequiredVersion: reqversion,
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

// ParseDir will parse Terramate configuration from a given directory,
// parsing all files with the suffixes .tm and .tm.hcl.
// Note: it does not recurse into child directories.
func ParseDir(dir string) (Config, error) {
	logger := log.With().
		Str("action", "ParseDir()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("Parsing configuration files")

	loadedParser, err := loadCfgBlocks(dir)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config files: %w", err)
	}

	logger.Trace().Msg("creating config from loaded parser")

	return newCfgFromParsedHCLs(dir, loadedParser)
}

// HCLBlocks maps a filename to a slice of blocks associated with it
type HCLBlocks map[string][]*hclsyntax.Block

// ParseGlobalsBlocks parses all Terramate files on the given dir, returning
// only global blocks (other blocks are discarded).
func ParseGlobalsBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "ParseGlobalsBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseHCLBlocks(dir, "globals", func(block *hclsyntax.Block) error {
		// Not validated with schema because cant find a way to validate
		// N arbitrary attributes (defined by user/dynamic).
		if len(block.Body.Blocks) > 0 {
			return errors.New("blocks inside globals are not allowed")
		}
		if len(block.Labels) > 0 {
			return fmt.Errorf("labels on globals block are not allowed, found %v", block.Labels)
		}
		return nil
	})
}

// ParseExportAsLocalsBlocks parses all Terramate files on the given dir, returning
// only export_as_locals blocks (other blocks are discarded).
// export_as_locals blocks are validated, so the caller can expect valid blocks only or an error.
func ParseExportAsLocalsBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseExportAsLocalsBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseHCLBlocks(dir, "export_as_locals", func(block *hclsyntax.Block) error {
		if len(block.Labels) != 0 {
			return fmt.Errorf(
				"exported_as_locals should not have labels but has %v",
				block.Labels,
			)
		}
		if len(block.Body.Blocks) != 0 {
			return errors.New("export_as_locals should not have blocks")
		}
		return nil
	})
}

// ParseGenerateHCLBlocks parses all Terramate files on the given dir, returning
// only generate_hcl blocks (other blocks are discarded).
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func ParseGenerateHCLBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseGenerateHCLBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "content",
				LabelNames: []string{},
			},
		},
	}

	return parseHCLBlocks(dir, "generate_hcl", func(block *hclsyntax.Block) error {
		// Don't seem like I can use hcl.BodySchema to check for any non-empty
		// label, only specific label values.
		if len(block.Labels) != 1 {
			return fmt.Errorf(
				"generate_hcl must have single label instead got %v",
				block.Labels,
			)
		}
		if block.Labels[0] == "" {
			return errors.New("generate_hcl label can't be empty")
		}
		// Schema check passes if no block is present, so check for amount of blocks
		if len(block.Body.Blocks) != 1 {
			return fmt.Errorf("generate_hcl must have one 'content' block, got %d blocks", len(block.Body.Blocks))
		}
		_, diags := block.Body.Content(schema)
		if diags.HasErrors() {
			return diags
		}
		return nil
	})
}

// CopyBody will copy the src body to the given target, evaluating attributes using the
// given evaluation context.
//
// Scoped traversals, like name.traverse, for unknown namespaces will be copied
// as is (original expression form, no evaluation).
//
// Returns an error if the evaluation fails.
func CopyBody(target *hclwrite.Body, src *hclsyntax.Body, evalctx *eval.Context) error {
	logger := log.With().
		Str("action", "CopyBody()").
		Logger()

	logger.Trace().Msg("Sorting attributes.")

	// Avoid generating code with random attr order (map iteration is random)
	attrs := sortedAttributes(src.Attributes)

	for _, attr := range attrs {
		logger := logger.With().
			Str("attrName", attr.Name).
			Logger()

		logger.Trace().Msg("evaluating.")

		val, err := evalctx.Eval(attr.Expr)
		if err != nil {
			logger.Trace().Msg("evaluation failed, checking if is unknown scope traversal.")

			scope, isUnknownTraversal := getUnknownScopeTraversal(attr.Expr, evalctx)
			if isUnknownTraversal {
				logger.Trace().Msg("Is unknown scope traversal, copying as traversal.")
				target.SetAttributeTraversal(attr.Name, scope.Traversal)
				continue
			}

			return fmt.Errorf("parsing attribute %q: %v", attr.Name, err)
		}

		logger.Trace().
			Str("attribute", attr.Name).
			Msg("Setting evaluated attribute.")

		target.SetAttributeValue(attr.Name, val)
	}

	logger.Trace().Msg("Append blocks.")

	for _, block := range src.Blocks {
		targetBlock := target.AppendNewBlock(block.Type, block.Labels)
		if block.Body == nil {
			continue
		}
		if err := CopyBody(targetBlock.Body(), block.Body, evalctx); err != nil {
			return err
		}
	}

	return nil
}

func getUnknownScopeTraversal(expr hclsyntax.Expression, evalctx *eval.Context) (*hclsyntax.ScopeTraversalExpr, bool) {
	logger := log.With().
		Str("action", "hcl.getUnknownScopeTraversal").
		Logger()

	scopeTraversal, ok := expr.(*hclsyntax.ScopeTraversalExpr)
	if !ok {
		logger.Trace().Msg("expression is not a scope traversal")
		return nil, false
	}

	for _, varTraversal := range hclsyntax.Variables(scopeTraversal) {
		if evalctx.HasNamespace(varTraversal.RootName()) {
			logger.Trace().Msg("expression has known namespace")
			return nil, false
		}
	}

	return scopeTraversal, true
}

func sortedAttributes(attrs hclsyntax.Attributes) []*hclsyntax.Attribute {
	names := make([]string, 0, len(attrs))

	for name := range attrs {
		names = append(names, name)
	}

	log.Trace().
		Str("action", "sortedAttributes()").
		Msg("Sort attributes.")
	sort.Strings(names)

	sorted := make([]*hclsyntax.Attribute, len(names))
	for i, name := range names {
		sorted[i] = attrs[name]
	}

	return sorted
}

func parseBlocksOfType(path string, blocktype string) ([]*hclsyntax.Block, error) {
	logger := log.With().
		Str("action", "parseBlocksOfType()").
		Str("path", path).
		Logger()

	logger.Trace().Msg("Get file info.")

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

	logger.Trace().Msg("Sort elements.")

	sort.Strings(elems)
	*target = elems
	return nil
}

func parseStack(stack *Stack, stackblock *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseStack()").
		Str("stack", stack.Name).
		Logger()

	logger.Debug().Msg("Get stack attributes.")

	for name, value := range stackblock.Body.Attributes {
		logger.Trace().Msg("Get attribute value.")

		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("failed to evaluate %q attribute: %w",
					name, diags),
			)
		}

		logger.Trace().
			Str("attribute", name).
			Msg("Setting attribute on configuration.")

		switch name {

		case "name":
			if attrVal.Type() != cty.String {
				return errutil.Chain(ErrMalformedTerramateConfig,
					fmt.Errorf("field stack.\"name\" must be a \"string\" but given %q",
						attrVal.Type().FriendlyName()),
				)
			}
			stack.Name = attrVal.AsString()

		case "after":
			err := assignSet(name, &stack.After, attrVal)
			if err != nil {
				return err
			}

		case "before":
			err := assignSet(name, &stack.Before, attrVal)
			if err != nil {
				return err
			}

		case "wants":
			err := assignSet(name, &stack.Wants, attrVal)
			if err != nil {
				return err
			}

		case "description":
			logger.Trace().Msg("parsing stack description.")
			if attrVal.Type() != cty.String {
				return errutil.Chain(ErrMalformedTerramateConfig,
					fmt.Errorf("field stack.\"description\" must be a \"string\" but given %q",
						attrVal.Type().FriendlyName()),
				)
			}
			stack.Description = attrVal.AsString()

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

	logger.Trace().Msg("Range over block attributes.")

	for name := range block.Body.Attributes {
		return errutil.Chain(ErrMalformedTerramateConfig,
			fmt.Errorf("unrecognized attribute terramate.config.%s", name),
		)
	}

	logger.Trace().Msg("Range over blocks.")

	for _, b := range block.Body.Blocks {
		switch b.Type {
		case "git":
			logger.Trace().Msg("Type was 'git'.")

			if cfg.Git != nil {
				return errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple terramate.config.git blocks"),
				)
			}

			cfg.Git = &GitConfig{}

			logger.Trace().Msg("Parse git config.")

			err := parseGitConfig(cfg.Git, b)
			if err != nil {
				return err
			}
		case "generate":
			logger.Trace().Msg("Found block generate")

			if cfg.Generate != nil {
				return errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("multiple terramate.config.backend blocks"),
				)
			}

			logger.Trace().Msg("Parsing terramate.config.generate.")

			gencfg, err := parseGenerateConfig(b)
			if err != nil {
				return errutil.Chain(ErrMalformedTerramateConfig, err)
			}
			cfg.Generate = &gencfg

		default:
			return errutil.Chain(ErrMalformedTerramateConfig,
				fmt.Errorf("unrecognized block type %q", b.Type),
			)
		}
	}

	return nil
}

func parseGenerateConfig(block *hclsyntax.Block) (GenerateConfig, error) {
	logger := log.With().
		Str("action", "parseGenerateConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	cfg := GenerateConfig{}

	for name, value := range block.Body.Attributes {
		logger := logger.With().
			Str("attribute", name).
			Logger()

		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return GenerateConfig{}, errutil.Chain(
				ErrHCLSyntax,
				fmt.Errorf("failed to evaluate terramate.config.generate.%s attribute: %w",
					name, diags),
			)
		}

		switch name {
		case "backend_config_filename":
			{
				logger.Trace().Msg("parsing")

				if attrVal.Type() != cty.String {
					return GenerateConfig{}, fmt.Errorf("terramate.config.generate.%s is not a string but %q",
						name,
						attrVal.Type().FriendlyName())
				}
				cfg.BackendCfgFilename = attrVal.AsString()

				logger.Trace().Msg("parsed with success")
			}
		case "locals_filename":
			{
				logger.Trace().Msg("parsing")

				if attrVal.Type() != cty.String {
					return GenerateConfig{}, fmt.Errorf("terramate.config.generate.%s is not a string but %q",
						name,
						attrVal.Type().FriendlyName())
				}
				cfg.LocalsFilename = attrVal.AsString()

				logger.Trace().Msg("parsed with success")
			}

		default:
			return GenerateConfig{}, fmt.Errorf("unrecognized attribute terramate.config.generate.%s", name)
		}
	}

	if cfg.LocalsFilename != "" && cfg.LocalsFilename == cfg.BackendCfgFilename {
		return GenerateConfig{}, fmt.Errorf(
			"terramate.config.generate: locals and backend cfg files have the same name %q", cfg.LocalsFilename)
	}

	return cfg, nil
}

func parseGitConfig(git *GitConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

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

	logger.Trace().Msg("Range over blocks.")

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
	case "terramate", "stack", "backend", "globals", "export_as_locals", "generate_hcl":
		logger.Trace().Msg("Block name was allowed.")
		return true
	default:
		return false
	}
}

func loadCfgBlocks(dir string) (*hclparse.Parser, error) {
	logger := log.With().
		Str("action", "loadCfgBlocks()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing files")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading files to load terramate cfg: %v", err)
	}

	logger.Trace().Msg("looking for Terramate files")

	parser := hclparse.NewParser()

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if dirEntry.IsDir() {
			logger.Trace().Msg("ignoring dir")
			continue
		}

		filename := dirEntry.Name()
		if strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl") {
			path := filepath.Join(dir, filename)

			logger.Trace().Msg("Reading config file.")

			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading Terramate config %q: %v", path, err)
			}

			logger.Trace().Msg("Parsing config.")

			_, diags := parser.ParseHCL(data, filename)
			if diags.HasErrors() {
				return nil, errutil.Chain(ErrHCLSyntax, diags)
			}

			logger.Trace().Msg("Terramate config file parsed successfully")
		}
	}

	return parser, nil
}

func newCfgFromParsedHCLs(dir string, parser *hclparse.Parser) (Config, error) {
	logger := log.With().
		Str("action", "newCfgFromParsedHCLs()").
		Str("dir", dir).
		Logger()

	tmconfig := Config{
		absdir: dir,
	}

	for fname, hclfile := range parser.Files() {
		logger := logger.With().
			Str("filename", fname).
			Logger()

		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)

		logger.Trace().Msg("checking for attributes.")

		for name := range body.Attributes {
			return Config{}, errutil.Chain(
				ErrMalformedTerramateConfig,
				fmt.Errorf("unrecognized attribute %q", name),
			)
		}

		var stackblock *hclsyntax.Block
		var tmblocks []*hclsyntax.Block
		var foundstack bool

		logger.Trace().Msg("Range over blocks.")

		for _, block := range body.Blocks {
			if !blockIsAllowed(block.Type) {
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("block type %q is not supported", block.Type),
				)
			}

			if block.Type == "terramate" {
				logger.Trace().Msg("Found 'terramate' block.")
				tmblocks = append(tmblocks, block)
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

		for _, tmblock := range tmblocks {
			logger.Trace().Msg("Found terramate block type.")

			if len(tmblock.Labels) > 0 {
				return Config{}, errutil.Chain(
					ErrMalformedTerramateConfig,
					fmt.Errorf("terramate block must have no labels"),
				)
			}

			if tmconfig.Terramate == nil {
				tmconfig.Terramate = &Terramate{}
			}

			tm := tmconfig.Terramate

			logger.Trace().Msg("Range over terramate block attributes.")

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
					logger.Trace().Msg("Parsing  attribute 'required_version'.")

					if attrVal.Type() != cty.String {
						return Config{}, errutil.Chain(
							ErrMalformedTerramateConfig,
							fmt.Errorf("attribute %q is not a string", name),
						)
					}
					if tm.RequiredVersion != "" {
						return Config{}, errutil.Chain(
							ErrMalformedTerramateConfig,
							fmt.Errorf("attribute %q is duplicated", name),
						)
					}
					tm.RequiredVersion = attrVal.AsString()

				default:
					return Config{}, errutil.Chain(ErrMalformedTerramateConfig,
						fmt.Errorf("invalid attribute %q", name),
					)
				}
			}

			logger.Trace().Msg("Range over terramate blocks")

			for _, block := range tmblock.Body.Blocks {
				switch block.Type {
				case "backend":
					logger.Trace().Msg("Parsing backend block.")

					if tm.Backend != nil {
						return Config{}, errutil.Chain(
							ErrMalformedTerramateConfig,
							fmt.Errorf("found second backend block in file %q, only one allowed", fname),
						)
					}

					if len(block.Labels) != 1 {
						return Config{}, errutil.Chain(
							ErrMalformedTerramateConfig,
							fmt.Errorf("backend type expects 1 label but given %v",
								block.Labels),
						)
					}
					tm.Backend = block

				case "config":
					logger.Trace().Msg("Found config block.")

					if tm.RootConfig == nil {
						tm.RootConfig = &RootConfig{}
					}

					logger.Trace().Msg("Parse root config.")

					err := parseRootConfig(tm.RootConfig, block)
					if err != nil {
						return Config{}, err
					}
				default:
					return Config{}, errutil.Chain(
						ErrMalformedTerramateConfig,
						fmt.Errorf("block type %q not supported", block.Type))
				}

			}
		}

		if !foundstack {
			continue
		}

		logger.Debug().Msg("Parsing stack cfg.")

		if tmconfig.Stack != nil {
			return Config{}, fmt.Errorf(
				"%w: found stack blocks in multiple files, only one block allowed",
				ErrMalformedTerramateConfig,
			)
		}

		tmconfig.Stack = &Stack{}
		err := parseStack(tmconfig.Stack, stackblock)
		if err != nil {
			return Config{}, err
		}
	}

	return tmconfig, nil
}

type blockValidator func(*hclsyntax.Block) error

func parseHCLBlocks(dir, blocktype string, validate blockValidator) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.parseHCLBlocks").
		Str("configdir", dir).
		Str("blocktype", blocktype).
		Logger()

	logger.Trace().Msg("loading config")

	parser, err := loadCfgBlocks(dir)
	if err != nil {
		return HCLBlocks{}, fmt.Errorf("parsing %q: %w", blocktype, err)
	}

	logger.Trace().Msg("Validating and filtering blocks")

	hclblocks := HCLBlocks{}

	for fname, hclfile := range parser.Files() {
		logger := logger.With().
			Str("filename", fname).
			Logger()

		logger.Trace().Msg("filtering blocks")
		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)
		blocks := filterBlocksByType(blocktype, body.Blocks)

		if len(blocks) == 0 {
			continue
		}

		logger.Trace().Msg("validating blocks")

		for _, block := range blocks {
			if err := validate(block); err != nil {
				return nil, fmt.Errorf("%q: %v", fname, err)
			}
		}

		logger.Trace().Msg("validated blocks")

		hclblocks[fname] = blocks
	}

	return hclblocks, nil
}

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}
