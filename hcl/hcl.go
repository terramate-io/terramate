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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned during the HCL parsing.
const (
	ErrHCLSyntax       errors.Kind = "HCL syntax error"
	ErrTerramateSchema errors.Kind = "terramate schema error"
	ErrTerraformSchema errors.Kind = "terraform schema error"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

// Config represents a Terramate configuration.
type Config struct {
	Terramate *Terramate
	Stack     *Stack

	// absdir is the absolute path to the configuration directory.
	absdir string
}

// RunConfig represents Terramate run configuration.
type RunConfig struct {
	Env *RunEnv
}

// RunEnv represents Terramate run environment.
type RunEnv struct {
	// Attributes is the collection of attribute definitions within the env block.
	Attributes hclsyntax.Attributes
}

// GitConfig represents Terramate Git configuration.
type GitConfig struct {
	DefaultBranchBaseRef string // DefaultBranchBaseRef is the baseRef when in default branch.
	DefaultBranch        string // DefaultBranch is the default branch.
	DefaultRemote        string // DefaultRemote is the default remote.
}

// RootConfig represents the root config block of a Terramate configuration.
type RootConfig struct {
	Git *GitConfig
	Run *RunConfig
}

// Terramate is the parsed "terramate" HCL block.
type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	// RootConfig is the configuration at the project root directory (commonly
	// the git directory).
	RootConfig *RootConfig
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

// Blocks maps a filename to a slice of HCL blocks associated with it
type Blocks map[string][]*hclsyntax.Block

// GenFileBlocks maps filenames to slices of parsed generated_file blocks
// found on the file.
type GenFileBlocks map[string][]GenFileBlock

// GenHCLBlocks maps filenames to slices of parsed generated_hcl blocks
// found on the file.
type GenHCLBlocks map[string][]GenHCLBlock

// GenHCLBlock represents a parsed generate_hcl block.
type GenHCLBlock struct {
	// Label of the block.
	Label string
	// Content block.
	Content *hclsyntax.Block
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
}

// GenFileBlock represents a parsed generate_file block
type GenFileBlock struct {
	// Label of the block
	Label string
	// Content attribute of the block
	Content *hclsyntax.Attribute
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
}

// PartialEvaluator represents an HCL partial evaluator
type PartialEvaluator func(hclsyntax.Expression) (hclwrite.Tokens, error)

// TerramateParser is an HCL parser tailored for Terramate configuration schema.
// As the Terramate configuration can span multiple files in the same directory,
// this API allows you to define the exact set of files (and contents) that are
// going to be included in the final configuration.
type TerramateParser struct {
	dir         string
	files       map[string][]byte // path=content
	parsedFiles []string
	hclparser   *hclparse.Parser
}

// NewTerramateParser creates a Terramate parser for the directory dir.
func NewTerramateParser(dir string) *TerramateParser {
	return &TerramateParser{
		dir:       dir,
		files:     map[string][]byte{},
		hclparser: hclparse.NewParser(),
	}
}

// addDir walks over all the files in the directory dir and add all .tm and
// .tm.hcl files to the parser.
func (p *TerramateParser) addDir(dir string) error {
	logger := log.With().
		Str("action", "parser.AddDir()").
		Str("dir", dir).
		Logger()

	tmFiles, err := listTerramateFiles(dir)
	if err != nil {
		return errors.E(err, "adding directory to terramate parser")
	}

	for _, filename := range tmFiles {
		path := filepath.Join(dir, filename)
		logger.Trace().
			Str("file", path).
			Msg("Reading config file.")

		data, err := os.ReadFile(path)
		if err != nil {
			return errors.E(err, "reading config file %q", path)
		}

		if err := p.AddFile(path, data); err != nil {
			return err
		}

		logger.Trace().Msg("file added")
	}

	return nil
}

// AddFile adds a file to the set of files to be parsed.
func (p *TerramateParser) AddFile(name string, data []byte) error {
	if !strings.HasPrefix(name, p.dir) {
		return errors.E("parser only allow files from directory %q", p.dir)
	}
	if _, ok := p.files[name]; ok {
		return errors.E(os.ErrExist, "adding file %q to the parser", name)
	}

	p.files[name] = data
	return nil
}

// Parse the previously added files and return either a Config or an error.
func (p *TerramateParser) Parse() (Config, error) {
	errs := errors.L()
	for _, name := range p.sortedFilenames() {
		data := p.files[name]
		_, diags := p.hclparser.ParseHCL(data, name)
		if diags.HasErrors() {
			errs.Append(errors.E(ErrHCLSyntax, diags))
			continue
		}

		p.parsedFiles = append(p.parsedFiles, name)
	}

	cfg, err := p.parseTerramateSchema()
	errs.Append(err)

	if err := errs.AsError(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (p *TerramateParser) sortedFilenames() []string {
	filenames := []string{}
	for fname := range p.files {
		filenames = append(filenames, fname)
	}
	sort.Strings(filenames)
	return filenames
}

// NewConfig creates a new HCL config with dir as config directory path.
func NewConfig(dir string) (Config, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return Config{}, errors.E(err, "initializing config")
	}

	if !st.IsDir() {
		return Config{}, errors.E("config constructor requires a directory path")
	}

	return Config{
		absdir: dir,
	}, nil
}

// AbsDir returns the absolute path of the configuration directory.
func (c Config) AbsDir() string { return c.absdir }

// IsEmpty returns true if the config is empty, false otherwise.
func (c Config) IsEmpty() bool {
	return c.Stack == nil && c.Terramate == nil
}

// Save the configuration file using filename inside config directory.
func (c Config) Save(filename string) (err error) {
	cfgpath := filepath.Join(c.absdir, filename)
	f, err := os.Create(cfgpath)
	if err != nil {
		return errors.E(err, "saving configuration file %q", cfgpath)
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

	logger.Trace().Msg("Get path information.")

	_, err := os.Stat(path)
	if err != nil {
		return nil, errors.E(err, "stat failed on %q", path)
	}

	logger.Trace().Msg("Create new parser")

	p := hclparse.NewParser()

	logger.Debug().Msg("Parse HCL file")

	errs := errors.L()

	f, diags := p.ParseHCLFile(path)
	errs.Append(diags)

	if err := errs.AsError(); err != nil {
		return nil, errors.E(ErrHCLSyntax, err)
	}

	body := f.Body.(*hclsyntax.Body)

	logger.Trace().Msg("Parse modules")

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

// ParseDir will parse Terramate configuration from a given directory,
// parsing all files with the suffixes .tm and .tm.hcl.
// Note: it does not recurse into child directories.
func ParseDir(dir string) (Config, error) {
	logger := log.With().
		Str("action", "ParseDir()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("Parsing configuration files")

	p := NewTerramateParser(dir)
	err := p.addDir(dir)
	if err != nil {
		return Config{}, errors.E("adding files to parser", err)
	}
	return p.Parse()
}

// ParseGlobalsBlocks parses all Terramate files on the given dir, returning
// only global blocks (other blocks are discarded).
func ParseGlobalsBlocks(dir string) (Blocks, error) {
	logger := log.With().
		Str("action", "ParseGlobalsBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseBlocks(dir, "globals", validateGlobalsBlock)
}

func validateGlobalsBlock(block *hclsyntax.Block) error {
	// Not validated with schema because cant find a way to validate
	// N arbitrary attributes (defined by user/dynamic).

	errs := errors.L()
	if len(block.Body.Blocks) > 0 {
		errs.Append(errors.E(block.Body.Blocks[0].Range(),
			"blocks inside globals are not allowed"))
	}
	if len(block.Labels) > 0 {
		errs.Append(errors.E(block.OpenBraceRange,
			"labels on globals block are not allowed, found %v",
			block.Labels))
	}
	return errs.AsError()
}

// ParseGenerateHCLBlocks parses all Terramate files on the given dir, returning
// only generate_hcl blocks (other blocks are discarded).
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func ParseGenerateHCLBlocks(dir string) (GenHCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseGenerateHCLBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	blocks, err := parseBlocks(dir, "generate_hcl", func(block *hclsyntax.Block) error {
		return validateGenerateHCLBlock(block)
	})
	if err != nil {
		return nil, err
	}

	res := GenHCLBlocks{}

	for filename, hclBlocks := range blocks {
		genHCLBlocks := make([]GenHCLBlock, len(hclBlocks))
		for i, hclBlock := range hclBlocks {
			genHCLBlocks[i] = GenHCLBlock{
				Label:     hclBlock.Labels[0],
				Content:   hclBlock.Body.Blocks[0],
				Condition: hclBlock.Body.Attributes["condition"],
			}
		}
		res[filename] = genHCLBlocks
	}

	return res, nil
}

// ParseGenerateFileBlocks parses all Terramate files on the given dir, returning
// parsed generate_file blocks.
func ParseGenerateFileBlocks(dir string) (GenFileBlocks, error) {
	blocks, err := parseBlocks(dir, "generate_file", func(block *hclsyntax.Block) error {
		return validateGenerateFileBlock(block)
	})
	if err != nil {
		return nil, err
	}

	res := GenFileBlocks{}

	for filename, fileBlocks := range blocks {
		genFileBlocks := make([]GenFileBlock, len(fileBlocks))
		for i, fileBlock := range fileBlocks {
			genFileBlocks[i] = GenFileBlock{
				Label:     fileBlock.Labels[0],
				Content:   fileBlock.Body.Attributes["content"],
				Condition: fileBlock.Body.Attributes["condition"],
			}
		}

		res[filename] = genFileBlocks
	}

	return res, nil
}

func validateGenerateHCLBlock(block *hclsyntax.Block) error {
	errs := errors.L()

	// Don't seem like we can use hcl.BodySchema to check for any non-empty
	// label, only specific label values.
	if len(block.Labels) != 1 {
		errs.Append(errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_hcl must have single label instead got %v",
			block.Labels,
		))
	} else if block.Labels[0] == "" {
		errs.Append(errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_hcl label can't be empty"))
	}
	// Schema check passes if no block is present, so check for amount of blocks
	if len(block.Body.Blocks) == 0 {
		errs.Append(errors.E(ErrTerramateSchema, block.Body.Range(),
			"generate_hcl must have one 'content' block"))
	} else if len(block.Body.Blocks) != 1 {
		errs.Append(errors.E(ErrTerramateSchema, block.Body.Range(),
			"generate_hcl must have one block of type 'content', found %d blocks",
			len(block.Body.Blocks)))
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "condition",
				Required: false,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "content",
				LabelNames: []string{},
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		errs.Append(errors.E(ErrHCLSyntax, diags))
	}
	return errs.AsError()
}

func validateGenerateFileBlock(block *hclsyntax.Block) error {
	errs := errors.L()
	if len(block.Labels) != 1 {
		errs.Append(errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_file must have single label instead got %v",
			block.Labels,
		))
	} else if block.Labels[0] == "" {
		errs.Append(errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_file label can't be empty"))
	}
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "content",
				Required: true,
			},
			{
				Name:     "condition",
				Required: false,
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		errs.Append(errors.E(ErrTerramateSchema, diags))
	}
	return errs.AsError()
}

// CopyBody will copy the src body to the given target, evaluating attributes using the
// given evaluation context.
//
// Scoped traversals, like name.traverse, for unknown namespaces will be copied
// as is (original expression form, no evaluation).
//
// Returns an error if the evaluation fails.
func CopyBody(target *hclwrite.Body, src *hclsyntax.Body, eval PartialEvaluator) error {
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
		tokens, err := eval(attr.Expr)
		if err != nil {
			return errors.E(err, attr.Expr.Range())
		}

		logger.Trace().Str("attribute", attr.Name).Msg("Setting evaluated attribute.")
		target.SetAttributeRaw(attr.Name, tokens)
	}

	logger.Trace().Msg("Append blocks.")

	for _, block := range src.Blocks {
		targetBlock := target.AppendNewBlock(block.Type, block.Labels)
		if block.Body == nil {
			continue
		}
		if err := CopyBody(targetBlock.Body(), block.Body, eval); err != nil {
			return err
		}
	}

	return nil
}

func sortedAttributes(attrs hclsyntax.Attributes) []*hclsyntax.Attribute {
	names := make([]string, 0, len(attrs))

	for name := range attrs {
		names = append(names, name)
	}

	log.Trace().Str("action", "sortedAttributes()").Msg("Sort attributes.")
	sort.Strings(names)

	sorted := make([]*hclsyntax.Attribute, len(names))
	for i, name := range names {
		sorted[i] = attrs[name]
	}

	return sorted
}

func findStringAttr(block *hclsyntax.Block, attrName string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().Msg("Range over attributes.")
	for _, attr := range sortedAttributes(block.Body.Attributes) {
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

func assignSet(name string, target *[]string, val cty.Value) error {
	logger := log.With().
		Str("action", "assignSet()").
		Logger()

	if val.IsNull() {
		return nil
	}

	// as the parser is schemaless it only creates tuples (lists of arbitrary types).
	// we have to check the elements themselves.
	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return errors.E(ErrTerramateSchema, "field %q must be a set(string) but "+
			"found a %q", name, val.Type().FriendlyName())
	}

	logger.Trace().Msg("Iterate over values.")

	errs := errors.L()
	values := map[string]struct{}{}
	iterator := val.ElementIterator()
	index := -1
	for iterator.Next() {
		index++
		_, elem := iterator.Element()

		logger.Trace().Msg("Check element is of correct type.")

		if elem.Type() != cty.String {
			errs.Append(errors.E("field %q must be a set(string) but element %d "+
				"has type %q", name, index, elem.Type().FriendlyName()))

			continue
		}

		logger.Trace().Msg("Get element as string.")

		str := elem.AsString()
		if _, ok := values[str]; ok {
			errs.Append(errors.E("duplicated entry %q in the index %d of field %q"+
				" of type set(string)", str, name))

			continue
		}
		values[str] = struct{}{}
	}

	if err := errs.AsError(); err != nil {
		return err
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

	errs := errors.L()

	for _, block := range stackblock.Body.Blocks {
		errs.Append(
			errors.E(block.TypeRange, "unrecognized block %q", block.Type),
		)
	}

	logger.Debug().Msg("Get stack attributes.")

	for _, attr := range sortedAttributes(stackblock.Body.Attributes) {
		logger.Trace().Msg("Get attribute value.")

		attrVal, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(
				errors.E(diags, "failed to evaluate %q attribute", attr.Name),
			)
		}

		logger.Trace().
			Str("attribute", attr.Name).
			Msg("Setting attribute on configuration.")

		switch attr.Name {
		case "name":
			if attrVal.Type() != cty.String {
				errs.Append(errors.E(attr.NameRange,
					"field stack.\"name\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			stack.Name = attrVal.AsString()

		case "after":
			errs.Append(assignSet(attr.Name, &stack.After, attrVal))

		case "before":
			errs.Append(assignSet(attr.Name, &stack.Before, attrVal))

		case "wants":
			errs.Append(assignSet(attr.Name, &stack.Wants, attrVal))

		case "description":
			logger.Trace().Msg("parsing stack description.")
			if attrVal.Type() != cty.String {
				errs.Append(errors.E(attr.Expr.Range(),
					"field stack.\"description\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName(),
				))

				continue
			}
			stack.Description = attrVal.AsString()

		default:
			errs.Append(errors.E(
				attr.NameRange, "unrecognized attribute stack.%q", attr.Name,
			))
		}
	}

	return errs.AsError()
}

func parseRootConfig(cfg *RootConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseRootConfig()").
		Logger()

	errs := errors.L()
	if len(block.Labels) != 0 {
		errs.Append(errors.E(block.LabelRanges[0],
			"config type expects 0 label but has %v", block.Labels,
		))
	}

	logger.Trace().Msg("Range over block attributes.")

	for _, attr := range sortedAttributes(block.Body.Attributes) {
		errs.Append(errors.E(attr.NameRange,
			"unrecognized attribute terramate.config.%s", attr.Name,
		))
	}

	logger.Trace().Msg("Range over blocks.")

	for _, b := range block.Body.Blocks {
		switch b.Type {
		case "git":
			logger.Trace().Msg("Type was 'git'.")

			if cfg.Git != nil {
				errs.Append(errors.E(ErrTerramateSchema, b.DefRange(),
					"multiple terramate.config.git blocks"),
				)
			}

			cfg.Git = &GitConfig{}

			logger.Trace().Msg("Parse git config.")

			errs.Append(parseGitConfig(cfg.Git, b))
		default:
			errs.Append(errors.E(ErrTerramateSchema, b.DefRange(),
				"unrecognized block type"))
		}
	}

	return errs.AsError()
}

func parseGitConfig(git *GitConfig, gitBlock *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	errs := errors.L()

	for _, block := range gitBlock.Body.Blocks {
		errs.Append(errors.E(block.TypeRange, "unrecognized block %q", block.Type))
	}

	for _, attr := range sortedAttributes(gitBlock.Body.Attributes) {
		attrVal, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.%s attribute", attr.Name,
			))

			continue
		}
		switch attr.Name {
		case "default_branch":
			logger.Trace().Msg("Attribute name was 'default_branch'.")

			if attrVal.Type() != cty.String {
				errs.Append(errors.E(attr.Expr.Range(),
					"terramate.config.git.branch is not a string but %q",
					attrVal.Type().FriendlyName(),
				))

				continue
			}

			git.DefaultBranch = attrVal.AsString()
		case "default_remote":
			logger.Trace().Msg("Attribute name was 'default_remote'.")

			if attrVal.Type() != cty.String {
				errs.Append(errors.E(attr.NameRange,
					"terramate.config.git.remote is not a string but %q",
					attrVal.Type().FriendlyName(),
				))

				continue
			}

			git.DefaultRemote = attrVal.AsString()

		case "default_branch_base_ref":
			logger.Trace().Msg("Attribute name was 'default_branch_base_ref.")

			if attrVal.Type() != cty.String {
				errs.Append(errors.E(attr.NameRange,
					"terramate.config.git.defaultBranchBaseRef is not a string but %q",
					attrVal.Type().FriendlyName(),
				))

				continue
			}

			git.DefaultBranchBaseRef = attrVal.AsString()

		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.git.%s",
				attr.Name,
			))
		}
	}
	return errs.AsError()
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

func isValidTopLevelBlock(name string) bool {
	logger := log.With().
		Str("action", "isValidTopLevelBlock()").
		Logger()

	switch name {
	case "terramate", "stack", "globals", "generate_hcl", "generate_file":
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

	filenames, err := listTerramateFiles(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to load config files")
	}

	parser := hclparse.NewParser()
	errs := errors.L()

	for _, filename := range filenames {
		logger := logger.With().
			Str("entryName", filename).
			Logger()

		path := filepath.Join(dir, filename)

		logger.Trace().Msg("Reading config file.")

		data, err := os.ReadFile(path)
		if err != nil {
			errs.Append(errors.E(err, "reading config file %q", path))
			continue
		}

		logger.Trace().Msg("Parsing config.")

		_, diags := parser.ParseHCL(data, path)
		if diags.HasErrors() {
			errs.Append(errors.E(ErrHCLSyntax, diags))
			continue
		}

		logger.Trace().Msg("Config file parsed successfully")
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return parser, nil
}

func (p *TerramateParser) parseTerramateSchema() (Config, error) {
	logger := log.With().
		Str("action", "parseTerramateSchema()").
		Str("dir", p.dir).
		Logger()

	tmconfig := Config{
		absdir: p.dir,
	}

	errs := errors.L()
	fileMap := p.hclparser.Files()
	for _, fname := range p.parsedFiles {
		hclfile := fileMap[fname]

		logger := logger.With().
			Str("filename", fname).
			Logger()

		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)

		logger.Trace().Msg("checking for attributes.")

		for _, attr := range sortedAttributes(body.Attributes) {
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %q", attr.Name))
		}

		var stackblock *hclsyntax.Block
		var tmblocks []*hclsyntax.Block
		var foundstack bool

		logger.Trace().Msg("Range over blocks.")

		errKind := ErrTerramateSchema
		for _, block := range body.Blocks {
			if !isValidTopLevelBlock(block.Type) {
				errs.Append(errors.E(errKind, block.DefRange(),
					"block type %q is not supported", block.Type))
				continue
			}

			if block.Type == "terramate" {
				logger.Trace().Msg("Found 'terramate' block.")

				tmblocks = append(tmblocks, block)
				continue
			}

			if block.Type == "stack" {
				logger.Trace().Msg("Found stack block type.")

				if foundstack {
					errs.Append(errors.E(errKind, block.DefRange(),
						"duplicated stack block"))
				}

				foundstack = true
				stackblock = block
			}

			if block.Type == "generate_hcl" {
				logger.Trace().Msg("Found \"generate_hcl\" block")

				err := validateGenerateHCLBlock(block)
				if err != nil {
					errs.Append(errors.E(errKind, err))
				}

				// TODO(i4k): generate_hcl must be part of the whole Config.
				// ignoring the block for now.
			}

			if block.Type == "generate_file" {
				logger.Trace().Msg("Found \"generate_file\" block")

				err := validateGenerateFileBlock(block)
				if err != nil {
					errs.Append(errors.E(errKind, err))
				}

				// TODO(katcipis): generate_file must be part of the whole Config.
				// ignoring the block for now.
			}

			if block.Type == "globals" {
				logger.Trace().Msg("Found \"globals\" block.")

				err := validateGlobalsBlock(block)
				if err != nil {
					errs.Append(errors.E(errKind, err))
				}
			}
		}

		for _, tmblock := range tmblocks {
			if len(tmblock.Labels) > 0 {
				errs.Append(errors.E(errKind, tmblock.LabelRanges,
					"terramate block should not have labels"))
			}

			if tmconfig.Terramate == nil {
				tmconfig.Terramate = &Terramate{}
			}

			tm := tmconfig.Terramate

			logger.Trace().Msg("Range over terramate block attributes.")

			for _, attr := range sortedAttributes(tmblock.Body.Attributes) {
				attrVal, diags := attr.Expr.Value(nil)
				if diags.HasErrors() {
					errs.Append(errors.E(errKind, diags))
				}
				switch attr.Name {
				case "required_version":
					logger.Trace().Msg("Parsing  attribute 'required_version'.")

					if attrVal.Type() != cty.String {
						errs.Append(errors.E(errKind, attr.Expr.Range(),
							"attribute is not a string"))

						continue
					}
					if tm.RequiredVersion != "" {
						errs.Append(errors.E(errKind, attr.NameRange,
							"duplicated attribute"))
					}
					tm.RequiredVersion = attrVal.AsString()

				default:
					errs.Append(errors.E(errKind, attr.NameRange,
						"unsupported attribute"))
				}
			}

			logger.Trace().Msg("Range over terramate blocks")

			for _, block := range tmblock.Body.Blocks {
				switch block.Type {
				case "config":
					logger.Trace().Msg("Found config block.")

					if tm.RootConfig == nil {
						tm.RootConfig = &RootConfig{}
					}

					logger.Trace().Msg("Parse root config.")

					err := parseRootConfig(tm.RootConfig, block)
					if err != nil {
						errs.Append(errors.E(errKind, err))
					}

				default:
					errs.Append(errors.E(errKind, block.DefRange(),
						"block not supported"))
				}

			}
		}

		if !foundstack {
			continue
		}

		logger.Debug().Msg("Parsing stack cfg.")

		if tmconfig.Stack != nil {
			errs.Append(errors.E(errKind, stackblock.DefRange(),
				"duplicated stack blocks across configs"))
		}

		tmconfig.Stack = &Stack{}
		err := parseStack(tmconfig.Stack, stackblock)
		if err != nil {
			errs.Append(errors.E(errKind, err))
		}
	}

	if err := errs.AsError(); err != nil {
		return Config{}, err
	}

	return tmconfig, nil
}

type blockValidator func(*hclsyntax.Block) error

func parseBlocks(dir, blocktype string, validate blockValidator) (Blocks, error) {
	logger := log.With().
		Str("action", "hcl.parseBlocks").
		Str("configdir", dir).
		Str("blocktype", blocktype).
		Logger()

	logger.Trace().Msg("loading config")

	errs := errors.L()

	parser, err := loadCfgBlocks(dir)
	errs.Append(errors.E(err, "parsing %q", blocktype))

	logger.Trace().Msg("Validating and filtering blocks")

	hclblocks := Blocks{}
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
				return nil, errors.E(err, "validation failed")
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

func listTerramateFiles(dir string) ([]string, error) {
	logger := log.With().
		Str("action", "listTerramateFiles()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing files")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate files")
	}

	logger.Trace().Msg("looking for Terramate files")

	files := []string{}

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if strings.HasPrefix(dirEntry.Name(), ".") {
			logger.Trace().Msg("ignoring dotfile")
			continue
		}

		if dirEntry.IsDir() {
			logger.Trace().Msg("ignoring dir")
			continue
		}

		filename := dirEntry.Name()
		if isTerramateFile(filename) {
			logger.Trace().Msg("Found Terramate file")
			files = append(files, filename)
		}
	}

	return files, nil
}

// listTerramateDirs lists Terramate dirs, which are any dirs
// except ones starting with ".".
func listTerramateDirs(dir string) ([]string, error) {
	logger := log.With().
		Str("action", "listTerramateDirs()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing dirs")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list Terramate dirs")
	}

	logger.Trace().Msg("looking for Terramate directories")

	dirs := []string{}

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if !dirEntry.IsDir() {
			logger.Trace().Msg("ignoring non-dir")
			continue
		}

		if strings.HasPrefix(dirEntry.Name(), ".") {
			logger.Trace().Msg("ignoring dotdir")
			continue
		}

		dirs = append(dirs, dirEntry.Name())
	}

	return dirs, nil
}

func isTerramateFile(filename string) bool {
	return strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl")
}
