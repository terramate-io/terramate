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
	"github.com/mineiros-io/terramate/hcl/eval"
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
	// absdir is the absolute path to the configuration directory.
	absdir    string
	Terramate *Terramate
	Stack     *Stack
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

// GenFileBlocks maps a filenames to slices of parsed generated_file blocks.
type GenFileBlocks map[string][]GenFileBlock

// GenFileBlock represents a parsed generate_file block
type GenFileBlock struct {
	// Label of the block
	Label string
	// Content attribute of the block
	Content hclsyntax.Expression
}

// TerramateParser is an HCL parser tailored for Terramate configuration schema.
// As the Terramate configuration can span multiple files in the same directory,
// this API allows you to define the exact set of files (and contents) that are
// going to be included in the final configuration.
type TerramateParser struct {
	dir       string
	files     map[string][]byte // path=content
	hclparser *hclparse.Parser
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

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return errors.E(err, "adding directory to terramate parser")
	}

	logger.Trace().Msg("looking for Terramate files")

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
	for name, data := range p.files {
		_, diags := p.hclparser.ParseHCL(data, name)
		if diags.HasErrors() {
			return Config{}, errors.E(ErrHCLSyntax, diags)
		}
	}

	return p.parseTerramateSchema()
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

	logger.Trace().
		Msg("Get path information.")
	_, err := os.Stat(path)
	if err != nil {
		return nil, errors.E(err, "stat failed on %q", path)
	}

	logger.Trace().
		Msg("Create new parser.")
	p := hclparse.NewParser()

	logger.Debug().
		Msg("Parse HCL file.")
	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errors.E(ErrHCLSyntax, diags)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	logger.Trace().Msg("Parse modules.")

	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}

		if len(block.Labels) != 1 {
			return nil, errors.E(ErrTerraformSchema, block.OpenBraceRange,
				"\"module\" block must have 1 label")
		}

		moduleName := block.Labels[0]

		logger.Trace().Msg("Get source attribute.")
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			return nil, errors.E(ErrTerraformSchema, err,
				"looking for module.%q.source attribute", moduleName)
		}
		if !ok {
			return nil, errors.E(ErrTerraformSchema,
				hcl.RangeBetween(block.OpenBraceRange, block.CloseBraceRange),
				"module must have a \"source\" attribute",
			)
		}

		modules = append(modules, Module{Source: source})
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
	if len(block.Body.Blocks) > 0 {
		return errors.E(block.Body.Blocks[0].Range(),
			"blocks inside globals are not allowed")
	}
	if len(block.Labels) > 0 {
		return errors.E(block.OpenBraceRange,
			"labels on globals block are not allowed, found %v", block.Labels)
	}
	return nil
}

// ParseGenerateHCLBlocks parses all Terramate files on the given dir, returning
// only generate_hcl blocks (other blocks are discarded).
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func ParseGenerateHCLBlocks(dir string) (Blocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseGenerateHCLBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseBlocks(dir, "generate_hcl", func(block *hclsyntax.Block) error {
		return validateGenerateHCLBlock(block)
	})
}

// ParseGenerateFileBlocks parses all Terramate files on the given dir, returning
// parsed generate_file blocks.
func ParseGenerateFileBlocks(dir string) (GenFileBlocks, error) {
	//logger := log.With().
	//Str("action", "hcl.ParseGenerateFileBlocks").
	//Str("configdir", dir).
	//Logger()

	return nil, nil
}

func validateGenerateHCLBlock(block *hclsyntax.Block) error {
	// Don't seem like we can use hcl.BodySchema to check for any non-empty
	// label, only specific label values.
	if len(block.Labels) != 1 {
		return errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_hcl must have single label instead got %v",
			block.Labels,
		)
	}
	if block.Labels[0] == "" {
		return errors.E(ErrTerramateSchema, block.OpenBraceRange,
			"generate_hcl label can't be empty")
	}
	// Schema check passes if no block is present, so check for amount of blocks
	if len(block.Body.Blocks) == 0 {
		return errors.E(ErrTerramateSchema, block.Body.Range(),
			"generate_hcl must have one 'content' block")
	}
	if len(block.Body.Blocks) != 1 {
		return errors.E(ErrTerramateSchema, block.Body.Range(),
			"generate_hcl must have one block of type 'content', found %d blocks",
			len(block.Body.Blocks))
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "content",
				LabelNames: []string{},
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return errors.E(ErrHCLSyntax, diags)
	}
	return nil
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
		tokens, err := evalctx.PartialEval(attr.Expr)
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
		if err := CopyBody(targetBlock.Body(), block.Body, evalctx); err != nil {
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

func findStringAttr(block *hclsyntax.Block, attr string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().Msg("Range over attributes.")
	for name, value := range block.Body.Attributes {
		if name != attr {
			continue
		}

		logger.Trace().Msg("Found attribute that we were looking for.")
		logger.Trace().Msg("Get attribute value.")
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return "", false, errors.E(diags)
		}

		logger.Trace().Msg("Check value type is correct.")
		if attrVal.Type() != cty.String {
			return "", false, errors.E(
				"attribute %q is not a string", attr, value.Expr.Range(),
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

	logger.Trace().
		Msg("Check val is correct type.")
	if val.Type().IsSetType() {
		return errors.E("attribute %q is not a set", name)
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
			return errors.E("field %q is a set(string) but contains %q",
				name, elem.Type().FriendlyName())
		}

		logger.Trace().Msg("Get element as string.")

		str := elem.AsString()
		if _, ok := values[str]; ok {
			return errors.E("duplicated entry %q in field %q of type set(string)",
				str, name)
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
			return errors.E(diags, "failed to evaluate %q attribute", name)
		}

		logger.Trace().Str("attribute", name).Msg("Setting attribute on configuration.")

		switch name {
		case "name":
			if attrVal.Type() != cty.String {
				return errors.E(value.NameRange,
					"field stack.\"name\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName())
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
				return errors.E(value.Expr.Range(),
					"field stack.\"description\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName(),
				)
			}
			stack.Description = attrVal.AsString()

		default:
			return errors.E(value.NameRange, "unrecognized attribute stack.%q", name)
		}
	}

	return nil
}

func parseRootConfig(cfg *RootConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseRootConfig()").
		Logger()

	if len(block.Labels) != 0 {
		return errors.E(block.LabelRanges[0],
			"config type expects 0 label but has %v", block.Labels,
		)
	}

	logger.Trace().Msg("Range over block attributes.")

	for name, nameVal := range block.Body.Attributes {
		return errors.E(nameVal.NameRange,
			"unrecognized attribute terramate.config.%s", name,
		)
	}

	logger.Trace().Msg("Range over blocks.")

	for _, b := range block.Body.Blocks {
		switch b.Type {
		case "git":
			logger.Trace().Msg("Type was 'git'.")

			if cfg.Git != nil {
				return errors.E(ErrTerramateSchema, b.DefRange(),
					"multiple terramate.config.git blocks")
			}

			cfg.Git = &GitConfig{}

			logger.Trace().Msg("Parse git config.")

			if err := parseGitConfig(cfg.Git, b); err != nil {
				return err
			}
		default:
			return errors.E(ErrTerramateSchema, b.DefRange(),
				"unrecognized block type")
		}
	}

	return nil
}

func parseGitConfig(git *GitConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	for name, value := range block.Body.Attributes {
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return errors.E(diags,
				"failed to evaluate terramate.config.%s attribute", name,
			)
		}
		switch name {
		case "default_branch":
			logger.Trace().Msg("Attribute name was 'default_branch'.")

			if attrVal.Type() != cty.String {
				return errors.E(value.Expr.Range(),
					"terramate.config.git.branch is not a string but %q",
					attrVal.Type().FriendlyName(),
				)
			}

			git.DefaultBranch = attrVal.AsString()
		case "default_remote":
			logger.Trace().Msg("Attribute name was 'default_remote'.")

			if attrVal.Type() != cty.String {
				return errors.E(value.NameRange,
					"terramate.config.git.remote is not a string but %q",
					attrVal.Type().FriendlyName(),
				)
			}

			git.DefaultRemote = attrVal.AsString()

		case "default_branch_base_ref":
			logger.Trace().Msg("Attribute name was 'default_branch_base_ref.")

			if attrVal.Type() != cty.String {
				return errors.E(value.NameRange,
					"terramate.config.git.defaultBranchBaseRef is not a string but %q",
					attrVal.Type().FriendlyName(),
				)
			}

			git.DefaultBranchBaseRef = attrVal.AsString()

		default:
			return errors.E(value.NameRange, "unrecognized attribute terramate.config.git.%s", name)
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
	case "terramate", "stack", "globals", "generate_hcl":
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
		return nil, errors.E(err, "reading dir to load config files")
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
				return nil, errors.E(err, "reading config file %q", path)
			}

			logger.Trace().Msg("Parsing config.")

			_, diags := parser.ParseHCL(data, path)
			if diags.HasErrors() {
				return nil, errors.E(ErrHCLSyntax, diags)
			}

			logger.Trace().Msg("Config file parsed successfully")
		}
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

	for fname, hclfile := range p.hclparser.Files() {
		logger := logger.With().
			Str("filename", fname).
			Logger()

		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)

		logger.Trace().Msg("checking for attributes.")

		for name, val := range body.Attributes {
			return Config{}, errors.E(ErrTerramateSchema, val.NameRange,
				"unrecognized attribute %q", name)
		}

		var stackblock *hclsyntax.Block
		var tmblocks []*hclsyntax.Block
		var foundstack bool

		logger.Trace().Msg("Range over blocks.")

		errKind := ErrTerramateSchema
		for _, block := range body.Blocks {
			if !blockIsAllowed(block.Type) {
				return Config{}, errors.E(errKind, block.DefRange(),
					"block type %q is not supported", block.Type)
			}

			if block.Type == "terramate" {
				logger.Trace().Msg("Found 'terramate' block.")

				tmblocks = append(tmblocks, block)
				continue
			}

			if block.Type == "stack" {
				logger.Trace().Msg("Found stack block type.")

				if foundstack {
					return Config{}, errors.E(errKind, block.DefRange(),
						"duplicated stack block")
				}

				foundstack = true
				stackblock = block
			}

			if block.Type == "generate_hcl" {
				logger.Trace().Msg("Found \"generate_hcl\" block")

				err := validateGenerateHCLBlock(block)
				if err != nil {
					return Config{}, errors.E(errKind, err)
				}

				// TODO(i4k): generate_hcl must be part of the whole Config.
				// ignoring the block for now.
			}

			if block.Type == "globals" {
				logger.Trace().Msg("Found \"globals\" block.")

				err := validateGlobalsBlock(block)
				if err != nil {
					return Config{}, errors.E(errKind, err)
				}
			}
		}

		for _, tmblock := range tmblocks {
			if len(tmblock.Labels) > 0 {
				return Config{}, errors.E(errKind, tmblock.LabelRanges,
					"terramate block should not have labels")
			}

			if tmconfig.Terramate == nil {
				tmconfig.Terramate = &Terramate{}
			}

			tm := tmconfig.Terramate

			logger.Trace().Msg("Range over terramate block attributes.")

			for name, value := range tmblock.Body.Attributes {
				attrVal, diags := value.Expr.Value(nil)
				if diags.HasErrors() {
					return Config{}, errors.E(errKind, diags)
				}
				switch name {
				case "required_version":
					logger.Trace().Msg("Parsing  attribute 'required_version'.")

					if attrVal.Type() != cty.String {
						return Config{}, errors.E(errKind, value.Expr.Range(),
							"attribute is not a string")
					}
					if tm.RequiredVersion != "" {
						return Config{}, errors.E(errKind, value.NameRange,
							"duplicated attribute")
					}
					tm.RequiredVersion = attrVal.AsString()

				default:
					return Config{}, errors.E(errKind, value.NameRange,
						"unsupported attribute")
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
						return Config{}, errors.E(errKind, err)
					}
				default:
					return Config{}, errors.E(errKind, block.DefRange(),
						"block not supported")
				}

			}
		}

		if !foundstack {
			continue
		}

		logger.Debug().Msg("Parsing stack cfg.")

		if tmconfig.Stack != nil {
			return Config{}, errors.E(errKind, stackblock.DefRange(),
				"duplicated stack blocks across configs")
		}

		tmconfig.Stack = &Stack{}
		err := parseStack(tmconfig.Stack, stackblock)
		if err != nil {
			return Config{}, errors.E(errKind, err)
		}
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

	parser, err := loadCfgBlocks(dir)
	if err != nil {
		return Blocks{}, errors.E(err, "parsing %q", blocktype)
	}

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
