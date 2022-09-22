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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Errors returned during the HCL parsing.
const (
	ErrHCLSyntax           errors.Kind = "HCL syntax error"
	ErrTerramateSchema     errors.Kind = "terramate schema error"
	ErrImport              errors.Kind = "import error"
	ErrUnexpectedTerramate errors.Kind = "`terramate` block is only allowed at the project root directory"
)

const (
	// StackBlockType name of the stack block type
	StackBlockType = "stack"

	// StackIDField name of the stack id field
	StackIDField = "id"
)

// Config represents a Terramate configuration.
type Config struct {
	Terramate *Terramate
	Stack     *Stack
	Vendor    *VendorConfig

	// absdir is the absolute path to the configuration directory.
	absdir string
}

// RunConfig represents Terramate run configuration.
type RunConfig struct {
	// CheckGenCode enables generated code is up-to-date check on run.
	CheckGenCode bool

	// Env contains environment definitions for run.
	Env *RunEnv
}

// RunEnv represents Terramate run environment.
type RunEnv struct {
	// Attributes is the collection of attribute definitions within the env block.
	Attributes ast.Attributes
}

// GitConfig represents Terramate Git configuration.
type GitConfig struct {
	// DefaultBranchBaseRef is the baseRef when in default branch.
	DefaultBranchBaseRef string

	// DefaultBranch is the default branch.
	DefaultBranch string

	// DefaultRemote is the default remote.
	DefaultRemote string

	// CheckUntracked enables untracked files checking.
	CheckUntracked bool

	// CheckUncommitted enables uncommitted files checking.
	CheckUncommitted bool

	// CheckRemote enables checking if local default branch is updated with remote.
	CheckRemote bool
}

// RootConfig represents the root config block of a Terramate configuration.
type RootConfig struct {
	Git *GitConfig
	Run *RunConfig
}

// ManifestDesc represents a parsed manifest description.
type ManifestDesc struct {
	// Files is a list of patterns that specify which files the manifest wants to include.
	Files []string

	// Excludes is a list of patterns that specify which files the manifest wants to exclude.
	Excludes []string
}

// ManifestConfig represents the manifest config block of a Terramate configuration.
type ManifestConfig struct {
	Default *ManifestDesc
}

// VendorConfig is the parsed "vendor" HCL block.
type VendorConfig struct {
	// Manifest is the parsed manifest block, if any.
	Manifest *ManifestConfig

	// Dir is the path where vendored projects will be stored.
	Dir string
}

// Terramate is the parsed "terramate" HCL block.
type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	// Config is the parsed config blocks.
	Config *RootConfig
}

// StackID represents the stack ID. Its zero value represents an undefined ID.
type StackID struct {
	id *string
}

// Stack is the parsed "stack" HCL block.
type Stack struct {
	// ID of the stack. If the ID is nil it indicates this stack has no ID.
	ID StackID

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

	// WantedBy is a list of non-duplicated stack entries that must select
	// this stack whenever they are selected.
	WantedBy []string

	// Watch is a list of files to be watched for changes.
	Watch []string
}

// GenHCLBlock represents a parsed generate_hcl block.
type GenHCLBlock struct {
	// Origin is the filename where this block is defined.
	Origin string
	// Label of the block.
	Label string
	// Lets is a block of local variables.
	Lets hclsyntax.Blocks
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Content block.
	Content *hclsyntax.Block
}

// GenFileBlock represents a parsed generate_file block
type GenFileBlock struct {
	// Origin is the filename where this block is defined.
	Origin string
	// Label of the block
	Label string
	// Lets is a block of local variables.
	Lets hclsyntax.Blocks
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Content attribute of the block
	Content *hclsyntax.Attribute
}

// Evaluator represents a Terramate evaluator
type Evaluator interface {
	// Eval evaluates the given expression returning a value.
	Eval(hcl.Expression) (cty.Value, error)

	// PartialEval partially evaluates the given expression returning the
	// tokens that form the result of the partial evaluation. Any unknown
	// namespace access are ignored and left as is, while known namespaces
	// are substituted by its value.
	PartialEval(hcl.Expression) (hclwrite.Tokens, error)

	// SetNamespace adds a new namespace, replacing any with the same name.
	SetNamespace(name string, values map[string]cty.Value)

	// DeleteNamespace deletes a namespace.
	DeleteNamespace(name string)
}

// TerramateParser is an HCL parser tailored for Terramate configuration schema.
// As the Terramate configuration can span multiple files in the same directory,
// this API allows you to define the exact set of files (and contents) that are
// going to be included in the final configuration.
type TerramateParser struct {
	Config   RawConfig
	Imported RawConfig

	rootdir   string
	dir       string
	files     map[string][]byte // path=content
	hclparser *hclparse.Parser
	evalctx   *eval.Context

	// parsedFiles stores a map of all parsed files
	parsedFiles map[string]parsedFile

	strict bool
	// if true, calling Parse() or MinimalParse() will fail.
	parsed bool
}

var stackIDRegex = regexp.MustCompile("^[a-zA-Z0-9_-]{1,64}$")

// NewStackID creates a new StackID with the given string as its id.
// It guarantees that the id passed is a valid StackID value,
// an error is returned otherwise.
func NewStackID(id string) (StackID, error) {
	if !stackIDRegex.MatchString(id) {
		return StackID{}, errors.E("Stack ID %q doesn't match %q", id, stackIDRegex)
	}
	return StackID{id: &id}, nil
}

// Value returns the ID string value and true if this StackID is defined,
// it returns "" and false otherwise.
func (s StackID) Value() (string, bool) {
	if s.id == nil {
		return "", false
	}
	return *s.id, true
}

// NewGitConfig creates a git configuration with proper default values.
func NewGitConfig() *GitConfig {
	return &GitConfig{
		CheckUntracked:   true,
		CheckUncommitted: true,
		CheckRemote:      true,
	}
}

// parsedFile tells the origin and kind of the parsedFile.
// The kind can be either internal or external, meaning the file was parsed
// by this parser or by another parser instance, respectively.
type parsedFile struct {
	kind   parsedKind
	origin string
}

type parsedKind int

const (
	_ parsedKind = iota
	internal
	external
)

type mergeHandler func(block *ast.Block) error

// NewTerramateParser creates a Terramate parser for the directory dir inside
// the root directory.
// The parser creates sub-parsers for parsing imports but keeps a list of all
// parsed files of all sub-parsers for detecting cycles and import duplications.
// Calling Parse() or MinimalParse() multiple times is an error.
func NewTerramateParser(rootdir string, dir string) (*TerramateParser, error) {
	logger := log.With().
		Str("action", "parser.NewTerramateParser()").
		Str("rootdir", rootdir).
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("Creating parser")

	_, err := os.Stat(dir)
	if err != nil {
		return nil, errors.E(err, "failed to stat directory %q", dir)
	}
	if !strings.HasPrefix(dir, rootdir) {
		return nil, errors.E("directory %q is not inside root %q", dir, rootdir)
	}

	evalctx, err := eval.NewContext(dir)
	if err != nil {
		return nil, errors.E(err, "failed to initialize the evaluation context")
	}

	return &TerramateParser{
		rootdir:     rootdir,
		dir:         dir,
		files:       map[string][]byte{},
		hclparser:   hclparse.NewParser(),
		Config:      NewRawConfig(),
		Imported:    NewRawConfig(),
		parsedFiles: make(map[string]parsedFile),
		evalctx:     evalctx,
	}, nil
}

// NewStrictTerramateParser is like NewTerramateParser but will fail instead of
// warn for harmless configuration mistakes.
func NewStrictTerramateParser(rootdir string, dir string) (*TerramateParser, error) {
	parser, err := NewTerramateParser(rootdir, dir)
	if err != nil {
		return nil, err
	}
	parser.strict = true
	return parser, nil
}

func (p *TerramateParser) addParsedFile(origin string, kind parsedKind, files ...string) {
	for _, file := range files {
		p.parsedFiles[file] = parsedFile{
			kind:   kind,
			origin: origin,
		}
	}
}

// AddDir walks over all the files in the directory dir and add all .tm and
// .tm.hcl files to the parser.
func (p *TerramateParser) AddDir(dir string) error {
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

		if err := p.AddFileContent(path, data); err != nil {
			return err
		}

		logger.Trace().Msg("file added")
	}

	return nil
}

// AddFile adds a file path to be parsed.
func (p *TerramateParser) AddFile(path string) error {
	if !strings.HasPrefix(path, p.dir) {
		return errors.E("parser only allow files from directory %q", p.dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.E("adding file %q to parser", path, err)
	}
	return p.AddFileContent(path, data)
}

// AddFileContent adds a file to the set of files to be parsed.
func (p *TerramateParser) AddFileContent(name string, data []byte) error {
	if !strings.HasPrefix(name, p.dir) {
		return errors.E("parser only allow files from directory %q", p.dir)
	}
	if _, ok := p.files[name]; ok {
		return errors.E(os.ErrExist, "adding file %q to the parser", name)
	}

	p.files[name] = data
	return nil
}

// ParseConfig parses and checks the schema of previously added files and
// return either a Config or an error.
func (p *TerramateParser) ParseConfig() (Config, error) {
	errs := errors.L()
	errs.Append(p.Parse())

	// TODO(i4k): don't validate schema here.
	// Changing this requires changes to the editor extensions / linters / etc.
	cfg, err := p.parseTerramateSchema()
	errs.Append(err)

	if err == nil {
		errs.Append(p.checkConfigSanity(cfg))
	}

	if err := errs.AsError(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Parse does the syntax parsing and merging of configurations but do not
// validate if the HCL schema is a valid Terramate configuration.
func (p *TerramateParser) Parse() error {
	if p.parsed {
		return errors.E("files already parsed")
	}
	defer func() { p.parsed = true }()

	errs := errors.L()
	errs.Append(p.parseSyntax())
	errs.Append(p.applyImports())
	errs.Append(p.mergeConfig())
	return errs.AsError()
}

// ParsedBodies returns a map of filename to the parsed hclsyntax.Body.
func (p *TerramateParser) ParsedBodies() map[string]*hclsyntax.Body {
	parsed := make(map[string]*hclsyntax.Body)
	bodyMap := p.hclparser.Files()
	for _, filename := range p.internalParsedFiles() {
		hclfile := bodyMap[filename]
		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		parsed[filename] = hclfile.Body.(*hclsyntax.Body)
	}
	return parsed
}

// Imports returns all import blocks.
func (p *TerramateParser) Imports() (ast.Blocks, error) {
	errs := errors.L()

	var imports ast.Blocks
	bodies := p.ParsedBodies()
	for _, origin := range p.sortedParsedFilenames() {
		body := bodies[origin]
		for _, rawBlock := range body.Blocks {
			if rawBlock.Type != "import" {
				continue
			}
			importBlock := ast.NewBlock(origin, rawBlock)
			err := validateImportBlock(importBlock)
			errs.Append(err)
			if err == nil {
				imports = append(imports, importBlock)
			}
		}
	}
	if err := errs.AsError(); err != nil {
		return nil, err
	}
	return imports, nil
}

func (p *TerramateParser) mergeConfig() error {
	errs := errors.L()

	bodies := p.ParsedBodies()
	for _, origin := range p.sortedParsedFilenames() {
		body := bodies[origin]

		errs.Append(p.Config.mergeAttrs(ast.NewAttributes(origin, body.Attributes)))
		errs.Append(p.Config.mergeBlocks(ast.NewBlocks(origin, body.Blocks)))
	}
	return errs.AsError()
}

func (p *TerramateParser) parseSyntax() error {
	errs := errors.L()
	for _, name := range p.sortedFilenames() {
		data := p.files[name]
		_, diags := p.hclparser.ParseHCL(data, name)
		if diags.HasErrors() {
			errs.Append(errors.E(ErrHCLSyntax, diags))
			continue
		}
		p.addParsedFile(p.dir, internal, name)
	}
	return errs.AsError()
}

func (p *TerramateParser) applyImports() error {
	importBlocks, err := p.Imports()
	if err != nil {
		return err
	}

	errs := errors.L()
	for _, importBlock := range importBlocks {
		errs.Append(p.handleImport(importBlock))
	}
	return errs.AsError()
}

func (p *TerramateParser) handleImport(importBlock *ast.Block) error {
	logger := log.With().
		Str("action", "parser.handleImport()").
		Str("rootdir", p.rootdir).
		Str("dir", p.dir).
		Logger()

	srcAttr := importBlock.Attributes["source"]
	srcVal, diags := srcAttr.Expr.Value(nil)
	if diags.HasErrors() {
		return errors.E(ErrTerramateSchema, srcAttr.Expr.Range(),
			"failed to evaluate import.source")
	}

	if srcVal.Type() != cty.String {
		return attrErr(srcAttr, "import.source must be a string")
	}

	src := srcVal.AsString()

	logger.Trace().Msgf("handling import.source=%s", src)

	srcBase := filepath.Base(src)
	srcDir := filepath.Dir(src)

	logger.Trace().Msgf("srcDir = %s", srcDir)

	if filepath.IsAbs(srcDir) { // project-path
		logger.Trace().Msg("is absolute")
		srcDir = filepath.Join(p.rootdir, srcDir)
	} else {
		logger.Trace().Msg("is relative")
		srcDir = filepath.Join(p.dir, srcDir)
	}

	logger.Trace().Msgf("import.source directory is %s", srcDir)

	if srcDir == p.dir {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"importing files in the same directory is not permitted")
	}

	if strings.HasPrefix(p.dir, srcDir) {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"importing files in the same tree is not permitted")
	}

	src = filepath.Join(srcDir, srcBase)

	if _, ok := p.parsedFiles[src]; ok {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"file %q already parsed", src)
	}

	importParser, err := NewTerramateParser(p.rootdir, srcDir)
	if err != nil {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			err, "failed to create sub parser")
	}

	err = importParser.AddFile(src)
	if err != nil {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			err)
	}
	importParser.addParsedFile(p.dir, external, p.internalParsedFiles()...)
	err = importParser.Parse()
	if err != nil {
		return err
	}
	errs := errors.L()
	for _, block := range importParser.Config.UnmergedBlocks {
		if block.Type == "stack" {
			errs.Append(
				errors.E(ErrImport, srcAttr.Expr.Range(),
					"import of stack block is not permitted"))
		}
	}

	errs.Append(p.Imported.Merge(importParser.Imported))
	errs.Append(p.Imported.Merge(importParser.Config))
	if err := errs.AsError(); err != nil {
		return errors.E(ErrImport, err, "failed to merge imported configuration")
	}

	p.addParsedFile(p.dir, external, src)
	return nil
}

func (p *TerramateParser) sortedFilenames() []string {
	filenames := []string{}
	for fname := range p.files {
		filenames = append(filenames, fname)
	}
	sort.Strings(filenames)
	return filenames
}

func (p *TerramateParser) sortedParsedFilenames() []string {
	filenames := append([]string{}, p.internalParsedFiles()...)
	sort.Strings(filenames)
	return filenames
}

func (p *TerramateParser) internalParsedFiles() []string {
	filenames := []string{}
	for fname, parsed := range p.parsedFiles {
		if parsed.kind == internal {
			filenames = append(filenames, fname)
		}
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

// HasRunEnv returns true if the config has a terramate.config.run.env block defined
func (c Config) HasRunEnv() bool {
	return c.Terramate != nil &&
		c.Terramate.Config != nil &&
		c.Terramate.Config.Run != nil &&
		c.Terramate.Config.Run.Env != nil
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

// ParseDir will parse Terramate configuration from a given directory,
// using root as project workspace, parsing all files with the suffixes .tm and
// .tm.hcl. It parses in non-strict mode for compatibility with older versions.
// Note: it does not recurse into child directories.
func ParseDir(root string, dir string) (Config, error) {
	logger := log.With().
		Str("action", "ParseDir()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("Parsing configuration files")

	p, err := NewTerramateParser(root, dir)
	if err != nil {
		return Config{}, err
	}
	err = p.AddDir(dir)
	if err != nil {
		return Config{}, errors.E("adding files to parser", err)
	}
	return p.ParseConfig()
}

// ParseGenerateHCLBlocks parses all Terramate files on the given dir, returning
// only generate_hcl blocks (other blocks are discarded).
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func ParseGenerateHCLBlocks(root, dir string) ([]GenHCLBlock, error) {
	logger := log.With().
		Str("action", "hcl.ParseGenerateHCLBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	blocks, err := parseUnmergedBlocks(root, dir, "generate_hcl", func(block *ast.Block) error {
		return validateGenerateHCLBlock(block)
	})
	if err != nil {
		return nil, err
	}

	var genhclBlocks []GenHCLBlock
	for _, block := range blocks {
		var (
			lets    hclsyntax.Blocks
			content *hclsyntax.Block
		)

		for _, b := range block.Body.Blocks {
			switch b.Type {
			case "lets":
				lets = append(lets, b)
			case "content":
				if content != nil {
					return nil, errors.E(b.Range(),
						"multiple generate_hcl.content blocks defined",
					)
				}
				content = b
			default:
				// already validated but sanity checks...
				panic("unreachable")
			}
		}

		genhclBlocks = append(genhclBlocks, GenHCLBlock{
			Origin:    block.Origin,
			Label:     block.Labels[0],
			Lets:      lets,
			Content:   content,
			Condition: block.Body.Attributes["condition"],
		})
	}

	return genhclBlocks, nil
}

// ParseGenerateFileBlocks parses all Terramate files on the given dir, returning
// parsed generate_file blocks.
func ParseGenerateFileBlocks(root, dir string) ([]GenFileBlock, error) {
	blocks, err := parseUnmergedBlocks(root, dir, "generate_file", func(block *ast.Block) error {
		return validateGenerateFileBlock(block)
	})
	if err != nil {
		return nil, err
	}

	var genfileBlocks []GenFileBlock
	for _, block := range blocks {
		for _, subBlock := range block.Body.Blocks {
			if len(subBlock.Body.Blocks) > 0 {
				return nil, errors.E(
					subBlock.Body.Blocks[0].Range(), "lets does not support blocks",
				)
			}
		}

		genfileBlocks = append(genfileBlocks, GenFileBlock{
			Origin:    block.Origin,
			Label:     block.Labels[0],
			Lets:      block.Body.Blocks,
			Content:   block.Body.Attributes["content"],
			Condition: block.Body.Attributes["condition"],
		})
	}

	return genfileBlocks, nil
}

func validateImportBlock(block *ast.Block) error {
	errs := errors.L()
	if len(block.Labels) != 0 {
		errs.Append(errors.E(ErrTerramateSchema, block.LabelRanges[0],
			"import must have no labels but got %v",
			block.Labels,
		))
	}
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "source",
				Required: true,
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		errs.Append(errors.E(ErrTerramateSchema, diags))
	}
	return errs.AsError()
}

func validateGenerateHCLBlock(block *ast.Block) error {
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
			"generate_hcl must have at least one 'content' block"))
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
			{
				Type:       "lets",
				LabelNames: []string{},
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		errs.Append(errors.E(ErrTerramateSchema, diags))
	}
	err := errs.AsError()
	if err != nil {
		return err
	}

	for _, b := range block.Blocks {
		if b.Type == "lets" {
			errs.Append(checkHasSubBlocks(b))
		}
	}
	return errs.AsError()
}

func validateGenerateFileBlock(block *ast.Block) error {
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
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "lets",
				LabelNames: []string{},
			},
		},
	}

	_, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		errs.Append(errors.E(ErrTerramateSchema, diags))
	}
	err := errs.AsError()
	if err != nil {
		return err
	}

	for _, b := range block.Blocks {
		if b.Type == "lets" {
			errs.Append(checkHasSubBlocks(b))
		}
	}
	return errs.AsError()
}

func assignSet(name string, target *[]string, val cty.Value) error {
	logger := log.With().
		Str("action", "hcl.assignSet()").
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
	var elems []string
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
		elems = append(elems, str)
	}

	if err := errs.AsError(); err != nil {
		return err
	}

	*target = elems
	return nil
}

// ValueAsStringList will convert the given cty.Value to a string list.
func ValueAsStringList(val cty.Value) ([]string, error) {
	if val.IsNull() {
		return nil, nil
	}

	// as the parser is schemaless it only creates tuples (lists of arbitrary types).
	// we have to check the elements themselves.
	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return nil, errors.E("value must be a set(string), got %q",
			val.Type().FriendlyName())
	}

	errs := errors.L()
	var elems []string
	iterator := val.ElementIterator()
	index := -1
	for iterator.Next() {
		index++
		_, elem := iterator.Element()

		if elem.Type() != cty.String {
			errs.Append(errors.E("value must be a set(string) but val[%d] = %q",
				index, elem.Type().FriendlyName()))
			continue
		}

		elems = append(elems, elem.AsString())
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return elems, nil
}

func parseStack(evalctx *eval.Context, stack *Stack, stackblock *ast.Block) error {
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

	for _, attr := range ast.SortRawAttributes(stackblock.Body.Attributes) {
		logger.Trace().Msg("Get attribute value.")

		attrVal, err := evalctx.Eval(attr.Expr)
		if err != nil {
			errs.Append(
				errors.E(err, "failed to evaluate %q attribute", attr.Name),
			)
			continue
		}

		logger.Trace().
			Str("attribute", attr.Name).
			Msg("Setting attribute on configuration.")

		switch attr.Name {
		case StackIDField:
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.id must be a string but is %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			id, err := NewStackID(attrVal.AsString())
			if err != nil {
				errs.Append(errors.E(attr.NameRange, err))
				continue
			}
			logger.Trace().
				Str(StackIDField, attrVal.AsString()).
				Msg("found valid stack ID definition")
			stack.ID = id
		case "name":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.name must be a string but given %q",
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

		case "wanted_by":
			errs.Append(assignSet(attr.Name, &stack.WantedBy, attrVal))

		case "watch":
			errs.Append(assignSet(attr.Name, &stack.Watch, attrVal))

		case "description":
			logger.Trace().Msg("parsing stack description.")
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
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

func checkNoAttributes(block *ast.Block) error {
	errs := errors.L()

	for _, got := range block.Attributes.SortedList() {
		errs.Append(errors.E(got.NameRange,
			"unrecognized attribute %s.%s", block.Type, got.Name,
		))
	}

	return errs.AsError()
}

func checkNoLabels(block *ast.Block) error {
	errs := errors.L()

	for i, label := range block.Labels {
		errs.Append(errors.E(ErrTerramateSchema,
			"block %s has unexpected label %s",
			block.Type,
			block.LabelRanges[i],
			label))
	}

	return errs.AsError()
}

func checkNoBlocks(block *ast.Block) error {
	errs := errors.L()

	for _, childBlock := range block.Blocks {
		errs.Append(errors.E(ErrTerramateSchema,
			childBlock.DefRange(),
			"unexpected block %s inside %s",
			childBlock.Type,
			block.Type),
		)
	}

	return errs.AsError()
}

func checkHasSubBlocks(block *ast.Block, blocks ...string) error {
	errs := errors.L()

	found := false
checkBlocks:
	for _, got := range block.Blocks {
		for _, want := range blocks {
			if want == got.Type {
				if found {
					errs.Append(errors.E(ErrTerramateSchema,
						got.DefRange(),
						"duplicated block %s",
						got.Type),
					)
					continue checkBlocks
				}
				found = true
				continue checkBlocks
			}
		}

		errs.Append(errors.E(ErrTerramateSchema,
			got.DefRange(),
			"unexpected block %s inside %s",
			got.Type,
			block.Type),
		)
	}

	return errs.AsError()
}

func parseVendorConfig(cfg *VendorConfig, vendor *ast.Block) error {
	logger := log.With().
		Str("action", "hcl.parseVendorConfig()").
		Logger()

	errs := errors.L()

	for _, attr := range vendor.Attributes {
		switch attr.Name {
		case "dir":
			attrVal, err := attr.Expr.Value(nil)
			if err != nil {
				errs.Append(errors.E(ErrTerramateSchema, err, attr.NameRange,
					"evaluating %s.%s", vendor.Type, attr.Name))
				continue
			}
			if attrVal.Type() != cty.String {
				errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
					"%s.%s must be string, got %s", vendor.Type, attr.Name, attrVal.Type,
				))
				continue
			}
			cfg.Dir = attrVal.AsString()
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", vendor.Type, attr.Name,
			))
		}
	}
	errs.Append(checkNoLabels(vendor))
	errs.Append(checkHasSubBlocks(vendor, "manifest"))

	if err := errs.AsError(); err != nil {
		return err
	}

	if len(vendor.Blocks) == 0 {
		return nil
	}

	manifestBlock := vendor.Blocks[0]

	errs.Append(checkNoAttributes(manifestBlock))
	errs.Append(checkNoLabels(manifestBlock))
	errs.Append(checkHasSubBlocks(manifestBlock, "default"))

	if err := errs.AsError(); err != nil {
		return err
	}

	logger.Trace().Msg("parsing vendor.manifest.default block")

	cfg.Manifest = &ManifestConfig{}

	if len(manifestBlock.Blocks) == 0 {
		return nil
	}

	defaultBlock := manifestBlock.Blocks[0]

	errs.Append(checkNoBlocks(defaultBlock))

	cfg.Manifest.Default = &ManifestDesc{}

	for _, attr := range defaultBlock.Attributes {
		switch attr.Name {
		case "files":
			attrVal, err := attr.Expr.Value(nil)
			if err != nil {
				errs.Append(err)
				continue
			}
			if err := assignSet(attr.Name, &cfg.Manifest.Default.Files, attrVal); err != nil {
				errs.Append(errors.E(err, attr.NameRange))
			}
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", defaultBlock.Type, attr.Name,
			))
		}
	}

	return errs.AsError()
}

func parseRootConfig(cfg *RootConfig, block *ast.MergedBlock) error {
	logger := log.With().
		Str("action", "parseRootConfig()").
		Logger()

	errs := errors.L()

	logger.Trace().Msg("Range over block attributes.")

	for _, attr := range block.Attributes.SortedList() {
		errs.Append(errors.E(attr.NameRange,
			"unrecognized attribute terramate.config.%s", attr.Name,
		))
	}

	errs.AppendWrap(ErrTerramateSchema, block.ValidateSubBlocks("git", "run"))

	gitBlock, ok := block.Blocks["git"]
	if ok {
		logger.Trace().Msg("Type is 'git'")

		cfg.Git = NewGitConfig()

		logger.Trace().Msg("Parse git config.")

		errs.Append(parseGitConfig(cfg.Git, gitBlock))
	}

	runBlock, ok := block.Blocks["run"]
	if ok {
		logger.Trace().Msg("Type is 'run'")

		cfg.Run = &RunConfig{
			CheckGenCode: true,
		}

		logger.Trace().Msg("Parse run config.")

		errs.Append(parseRunConfig(cfg.Run, runBlock))
	}

	return errs.AsError()
}

func parseRunConfig(runCfg *RunConfig, runBlock *ast.MergedBlock) error {
	logger := log.With().
		Str("action", "parseRunConfig()").
		Logger()

	logger.Trace().Msg("Checking run.env block")

	errs := errors.L()
	for _, attr := range runBlock.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.run.%s attribute", attr.Name,
			))

			continue
		}

		switch attr.Name {
		case "check_gen_code":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.run.check_gen_code is not a bool but %q",
					value.Type().FriendlyName(),
				))

				continue
			}
			runCfg.CheckGenCode = value.True()
		default:
			errs.Append(errors.E("unrecognized attribute terramate.config.run.env.%s",
				attr.Name))
		}
	}

	errs.AppendWrap(ErrTerramateSchema, runBlock.ValidateSubBlocks("env"))

	block, ok := runBlock.Blocks["env"]
	if ok {
		runCfg.Env = &RunEnv{}
		errs.Append(parseRunEnv(runCfg.Env, block))
	}

	return errs.AsError()
}

func parseRunEnv(runEnv *RunEnv, envBlock *ast.MergedBlock) error {
	if len(envBlock.Attributes) > 0 {
		runEnv.Attributes = envBlock.Attributes
	}

	errs := errors.L()
	errs.AppendWrap(ErrTerramateSchema, envBlock.ValidateSubBlocks())
	return errs.AsError()
}

func parseGitConfig(git *GitConfig, gitBlock *ast.MergedBlock) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	errs := errors.L()

	errs.AppendWrap(ErrTerramateSchema, gitBlock.ValidateSubBlocks())

	for _, attr := range gitBlock.Attributes.SortedList() {
		logger := logger.With().
			Str("attribute", attr.Name).
			Logger()

		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.%s attribute", attr.Name,
			))
			continue
		}

		logger.Trace().Msg("setting attribute on config")

		switch attr.Name {
		case "default_branch":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.git.branch is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			git.DefaultBranch = value.AsString()
		case "default_remote":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.git.remote is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			git.DefaultRemote = value.AsString()

		case "default_branch_base_ref":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.git.defaultBranchBaseRef is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}
			git.DefaultBranchBaseRef = value.AsString()

		case "check_untracked":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_untracked is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			git.CheckUntracked = value.True()
		case "check_uncommitted":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_uncommitted is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			git.CheckUncommitted = value.True()
		case "check_remote":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_remote is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			git.CheckRemote = value.True()

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

func (p *TerramateParser) parseTerramateSchema() (Config, error) {
	logger := log.With().
		Str("action", "parseTerramateSchema()").
		Str("dir", p.dir).
		Logger()

	config := Config{
		absdir: p.dir,
	}

	errKind := ErrTerramateSchema
	errs := errors.L()

	logger.Trace().Msg("checking for top-level attributes.")

	rawconfig := p.Imported.Copy()
	err := rawconfig.Merge(p.Config)
	if err != nil {
		err = errors.E(err, ErrImport)
		errs.Append(err)
	}

	for _, attr := range rawconfig.MergedAttributes.SortedList() {
		errs.Append(errors.E(errKind, attr.NameRange,
			"unrecognized attribute %q", attr.Name))
	}

	logger.Trace().Msg("Range over unmerged blocks.")

	var foundstack, foundVendor bool
	var stackblock, vendorBlock *ast.Block

	for _, block := range rawconfig.UnmergedBlocks {
		// unmerged blocks

		logger := logger.With().
			Str("block", block.Type).
			Logger()

		switch block.Type {
		case StackBlockType:
			logger.Trace().Msg("Found stack block type.")

			if foundstack {
				errs.Append(errors.E(errKind, block.DefRange(),
					"duplicated stack block"))
				continue
			}

			foundstack = true
			stackblock = block
		case "vendor":
			logger.Trace().Msg("found vendor block")

			if foundVendor {
				errs.Append(errors.E(errKind, block.DefRange(),
					"duplicated vendor block"))
				continue
			}

			foundVendor = true
			vendorBlock = block

		case "generate_hcl":
			logger.Trace().Msg("Found \"generate_hcl\" block")
			errs.Append(validateGenerateHCLBlock(block))

		case "generate_file":
			logger.Trace().Msg("Found \"generate_file\" block")
			errs.Append(validateGenerateFileBlock(block))
		}
	}

	tmBlock, ok := rawconfig.MergedBlocks["terramate"]
	if ok {
		var tmconfig Terramate
		tmconfig, err := parseTerramateBlock(tmBlock)
		errs.Append(err)
		if err == nil {
			config.Terramate = &tmconfig
		}
	}

	if foundVendor {
		logger.Debug().Msg("parsing manifest")

		if config.Vendor != nil {
			errs.Append(errors.E(errKind, vendorBlock.DefRange(),
				"duplicated vendor blocks across configs"))
		}
		config.Vendor = &VendorConfig{}
		err := parseVendorConfig(config.Vendor, vendorBlock)
		if err != nil {
			errs.Append(errors.E(errKind, err))
		}
	}

	globalsBlock, ok := rawconfig.MergedBlocks["globals"]
	if ok {
		errs.AppendWrap(ErrTerramateSchema, globalsBlock.ValidateSubBlocks())

		// value ignored in the main parser.
	}

	if foundstack {
		logger.Debug().Msg("Parsing stack cfg.")

		if config.Stack != nil {
			errs.Append(errors.E(errKind, stackblock.DefRange(),
				"duplicated stack blocks across configs"))
		}

		config.Stack = &Stack{}
		errs.AppendWrap(errKind, parseStack(p.evalctx, config.Stack, stackblock))
	}

	if err := errs.AsError(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (p *TerramateParser) checkConfigSanity(cfg Config) error {
	logger := log.With().
		Str("action", "TerramateParser.checkConfigSanity()").
		Logger()

	rawconfig := p.Imported.Copy()
	_ = rawconfig.Merge(p.Config)

	errs := errors.L()
	tmblock := rawconfig.MergedBlocks["terramate"]
	if tmblock != nil && p.dir != p.rootdir {
		for _, raworigin := range tmblock.RawOrigins {
			if filepath.Dir(raworigin.Origin) != p.dir {
				errs.Append(
					errors.E(ErrUnexpectedTerramate, raworigin.TypeRange,
						"imported from directory %q", p.dir),
				)
			} else {
				errs.Append(
					errors.E(ErrUnexpectedTerramate, raworigin.TypeRange),
				)
			}
		}
	}
	if p.strict {
		return errs.AsError()
	}
	for _, err := range errs.Errors() {
		logger.Warn().Err(err).Send()
	}
	return nil
}

func parseTerramateBlock(block *ast.MergedBlock) (Terramate, error) {
	logger := log.With().
		Str("action", "parseTerramateBlock").
		Logger()

	logger.Trace().Msg("Range over terramate block attributes.")

	tm := Terramate{}

	errKind := ErrTerramateSchema
	errs := errors.L()
	for _, attr := range block.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(errKind, diags))
		}
		switch attr.Name {
		case "required_version":
			logger.Trace().Msg("Parsing  attribute 'required_version'.")

			if value.Type() != cty.String {
				errs.Append(errors.E(errKind, attr.Expr.Range(),
					"attribute is not a string"))

				continue
			}
			if tm.RequiredVersion != "" {
				errs.Append(errors.E(errKind, attr.NameRange,
					"duplicated attribute"))
			}
			tm.RequiredVersion = value.AsString()

		default:
			errs.Append(errors.E(errKind, attr.NameRange,
				"unsupported attribute"))
		}
	}

	errs.AppendWrap(ErrTerramateSchema, block.ValidateSubBlocks("config"))

	logger.Trace().Msg("Parse terramate sub blocks")

	configBlock, ok := block.Blocks["config"]
	if ok {
		logger.Trace().Msg("Found config block.")

		tm.Config = &RootConfig{}

		logger.Trace().Msg("Parse root config.")

		err := parseRootConfig(tm.Config, configBlock)
		if err != nil {
			errs.Append(errors.E(errKind, err))
		}
	}

	if err := errs.AsError(); err != nil {
		return Terramate{}, err
	}
	return tm, nil
}

type blockValidator func(*ast.Block) error

func parseUnmergedBlocks(root, dir, blocktype string, validate blockValidator) (ast.Blocks, error) {
	logger := log.With().
		Str("action", "hcl.parseBlocks").
		Str("configdir", dir).
		Str("blocktype", blocktype).
		Logger()

	logger.Trace().Msg("loading config")

	parser, err := NewTerramateParser(root, dir)
	if err != nil {
		return nil, err
	}
	err = parser.AddDir(dir)
	if err != nil {
		return nil, errors.E("adding files to parser", err)
	}

	err = parser.Parse()
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Validating and filtering blocks")

	var blocks ast.Blocks
	blocks = append(blocks,
		parser.Imported.filterUnmergedBlocksByType(blocktype)...)
	blocks = append(blocks,
		parser.Config.filterUnmergedBlocksByType(blocktype)...)
	for _, block := range blocks {
		if err := validate(block); err != nil {
			return nil, errors.E(err, "validation failed")
		}
	}

	logger.Trace().Msg("validated blocks")

	return blocks, nil
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

func hclAttrErr(attr *hclsyntax.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrTerramateSchema, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}

func attrErr(attr ast.Attribute, msg string, args ...interface{}) error {
	return hclAttrErr(attr.Attribute, msg, args...)
}
