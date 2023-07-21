// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/stdlib"
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
)

// Config represents a Terramate configuration.
type Config struct {
	Terramate *Terramate
	Stack     *Stack
	Globals   ast.MergedLabelBlocks
	Vendor    *VendorConfig
	Asserts   []AssertConfig
	Generate  GenerateConfig

	Imported RawConfig

	// absdir is the absolute path to the configuration directory.
	absdir string
}

// GenerateConfig includes code generation related configurations, like
// generate_file and generate_hcl.
type GenerateConfig struct {
	Files []GenFileBlock
	HCLs  []GenHCLBlock
}

// AssertConfig represents Terramate assert configuration block.
type AssertConfig struct {
	Range     info.Range
	Warning   hcl.Expression
	Assertion hcl.Expression
	Message   hcl.Expression
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

	// RequiredVersionAllowPreReleases allows pre-release to be matched if true.
	RequiredVersionAllowPreReleases bool

	// Config is the parsed config blocks.
	Config *RootConfig
}

// Stack is the parsed "stack" HCL block.
type Stack struct {
	// ID of the stack. If the ID is empty it indicates this stack has no ID.
	ID string

	// Name of the stack
	Name string

	// Description of the stack
	Description string

	// Tags is a list of non-duplicated list of tags
	Tags []string

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
	// Range is the range of the entire block definition.
	Range info.Range
	// Label of the block.
	Label string
	// Lets is a block of local variables.
	Lets *ast.MergedBlock
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Content block.
	Content *hclsyntax.Block
	// Asserts represents all assert blocks
	Asserts []AssertConfig
}

// GenFileBlock represents a parsed generate_file block
type GenFileBlock struct {
	// Range is the range of the entire block definition.
	Range info.Range
	// Label of the block
	Label string
	// Lets is a block of local variables.
	Lets *ast.MergedBlock
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Content attribute of the block
	Content *hclsyntax.Attribute
	// Context of the generation (stack by default).
	Context string
	// Asserts represents all assert blocks
	Asserts []AssertConfig
}

// Evaluator represents a Terramate evaluator
type Evaluator interface {
	// Eval evaluates the given expression returning a value.
	Eval(hcl.Expression) (cty.Value, error)

	// PartialEval partially evaluates the given expression returning the
	// tokens that form the result of the partial evaluation. Any unknown
	// namespace access are ignored and left as is, while known namespaces
	// are substituted by its value.
	PartialEval(hcl.Expression) (hcl.Expression, error)

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

// NewTerramateParser creates a Terramate parser for the directory dir inside
// the root directory.
// The parser creates sub-parsers for parsing imports but keeps a list of all
// parsed files of all sub-parsers for detecting cycles and import duplications.
// Calling Parse() or MinimalParse() multiple times is an error.
func NewTerramateParser(rootdir string, dir string) (*TerramateParser, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, errors.E(err, "failed to stat directory %q", dir)
	}
	if !strings.HasPrefix(dir, rootdir) {
		return nil, errors.E("directory %q is not inside root %q", dir, rootdir)
	}

	if !st.IsDir() {
		return nil, errors.E("%s is not a directory", dir)
	}

	return &TerramateParser{
		rootdir:     rootdir,
		dir:         dir,
		files:       map[string][]byte{},
		hclparser:   hclparse.NewParser(),
		Config:      NewTopLevelRawConfig(),
		Imported:    NewTopLevelRawConfig(),
		parsedFiles: make(map[string]parsedFile),
		evalctx:     eval.NewContext(stdlib.Functions(dir)),
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

	tmFiles, err := fs.ListTerramateFiles(dir)
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
			importBlock := ast.NewBlock(p.rootdir, rawBlock)
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

		errs.Append(p.Config.mergeAttrs(ast.NewAttributes(p.rootdir, ast.AsHCLAttributes(body.Attributes))))
		errs.Append(p.Config.mergeBlocks(ast.NewBlocks(p.rootdir, body.Blocks)))
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
	srcBase := path.Base(src)
	srcDir := path.Dir(src)
	if path.IsAbs(srcDir) { // project-path
		srcDir = filepath.Join(p.rootdir, srcDir)
	} else {
		srcDir = filepath.Join(p.dir, srcDir)
	}

	if srcDir == p.dir {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"importing files in the same directory is not permitted")
	}

	if strings.HasPrefix(p.dir, srcDir) {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"importing files in the same tree is not permitted")
	}

	src = filepath.Join(srcDir, srcBase)
	matches, err := filepath.Glob(src)
	if err != nil {
		return errors.E(ErrTerramateSchema, srcAttr.Expr.Range(),
			"failed to evaluate import.source")
	}
	if matches == nil {
		return errors.E(ErrImport, srcAttr.Expr.Range(),
			"import path %q returned no matches", srcVal.AsString())
	}
	for _, file := range matches {
		if _, ok := p.parsedFiles[file]; ok {
			return errors.E(ErrImport, srcAttr.Expr.Range(),
				"file %q already parsed", file)
		}

		st, err := os.Lstat(file)
		if err != nil {
			return errors.E(
				ErrImport,
				srcAttr.Expr.Range(),
				"failed to stat file %q",
				file,
			)
		}

		if st.IsDir() {
			return errors.E(
				ErrImport,
				srcAttr.Expr.Range(),
				"import directory is not allowed: %s",
				file,
			)
		}

		fileDir := filepath.Dir(file)
		importParser, err := NewTerramateParser(p.rootdir, fileDir)
		if err != nil {
			return errors.E(ErrImport, srcAttr.Expr.Range(),
				err, "failed to create sub parser: %s", fileDir)
		}

		err = importParser.AddFile(file)
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

		p.addParsedFile(p.dir, external, file)
	}
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

func (p *TerramateParser) parseStack(stackblock *ast.Block) (*Stack, error) {
	logger := log.With().
		Str("action", "parseStack()").
		Logger()

	errs := errors.L()
	for _, block := range stackblock.Body.Blocks {
		errs.Append(
			errors.E(block.TypeRange, "unrecognized block %q", block.Type),
		)
	}

	stack := &Stack{}

	logger.Debug().Msg("Get stack attributes.")
	attrs := ast.AsHCLAttributes(stackblock.Body.Attributes)
	for _, attr := range ast.SortRawAttributes(attrs) {
		logger.Trace().Msg("Get attribute value.")

		attrVal, err := p.evalctx.Eval(attr.Expr)
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
		case "id":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.id must be a string but is %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			stack.ID = attrVal.AsString()
		case "name":
			if attrVal.Type() != cty.String {
				errs.Append(hclAttrErr(attr,
					"field stack.name must be a string but given %q",
					attrVal.Type().FriendlyName()),
				)
				continue
			}
			stack.Name = attrVal.AsString()

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

			// The `tags`, `after`, `before`, `wants`, `wanted_by` and `watch`
			// have all the same parsing rules.
			// By the spec, they must be a `set(string)`.

			// In order to speed up the tests, only the `after` attribute is
			// extensively tested for all error cases.
			// **So have this in mind if the specification of any of the attributes
			// below change in the future**.

		case "tags":
			errs.Append(assignSet(attr.Name, &stack.Tags, attrVal))

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

		default:
			errs.Append(errors.E(
				attr.NameRange, "unrecognized attribute stack.%q", attr.Name,
			))
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return stack, nil
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
		absdir:   dir,
		Imported: NewTopLevelRawConfig(),
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
	return c.Stack == nil && c.Terramate == nil &&
		c.Vendor == nil && len(c.Asserts) == 0 &&
		len(c.Globals) == 0 &&
		len(c.Generate.Files) == 0 && len(c.Generate.HCLs) == 0
}

// HasGlobals tells if the configuration has any globals defined.
func (c Config) HasGlobals() bool {
	return len(c.Globals) > 0
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

// parseGenerateHCLBlock the generate_hcl block.
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func parseGenerateHCLBlock(block *ast.Block) (GenHCLBlock, error) {
	var (
		content *hclsyntax.Block
		asserts []AssertConfig
	)

	err := validateGenerateHCLBlock(block)
	if err != nil {
		return GenHCLBlock{}, err
	}

	letsConfig := NewCustomRawConfig(map[string]mergeHandler{
		"lets": (*RawConfig).mergeLabeledBlock,
	})

	errs := errors.L()
	for _, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "lets":
			errs.AppendWrap(ErrTerramateSchema, letsConfig.mergeBlocks(ast.Blocks{subBlock}))
		case "assert":
			assertCfg, err := parseAssertConfig(subBlock)
			if err != nil {
				errs.Append(err)
				continue
			}
			asserts = append(asserts, assertCfg)
		case "content":
			if content != nil {
				errs.Append(errors.E(subBlock.Range,
					"multiple generate_hcl.content blocks defined",
				))
				continue
			}
			content = subBlock.Block
		default:
			// already validated but sanity checks...
			panic(errors.E(errors.ErrInternal, "unexpected block type %s", subBlock.Type))
		}
	}

	if content == nil {
		errs.Append(
			errors.E(ErrTerramateSchema, `"generate_hcl" block requires a content block`, block.Range))
	}

	mergedLets := ast.MergedLabelBlocks{}
	for labelType, mergedBlock := range letsConfig.MergedLabelBlocks {
		if labelType.Type == "lets" {
			mergedLets[labelType] = mergedBlock

			errs.AppendWrap(ErrTerramateSchema, validateLets(mergedBlock))
		}
	}

	if err := errs.AsError(); err != nil {
		return GenHCLBlock{}, err
	}

	lets, ok := mergedLets[ast.NewEmptyLabelBlockType("lets")]
	if !ok {
		lets = ast.NewMergedBlock("lets", []string{})
	}

	return GenHCLBlock{
		Range:     block.Range,
		Label:     block.Labels[0],
		Lets:      lets,
		Asserts:   asserts,
		Content:   content,
		Condition: block.Body.Attributes["condition"],
	}, nil
}

// parseGenerateFileBlock parses all Terramate files on the given dir, returning
// parsed generate_file blocks.
func parseGenerateFileBlock(block *ast.Block) (GenFileBlock, error) {
	err := validateGenerateFileBlock(block)
	if err != nil {
		return GenFileBlock{}, err
	}

	var asserts []AssertConfig

	letsConfig := NewCustomRawConfig(map[string]mergeHandler{
		"lets": (*RawConfig).mergeLabeledBlock,
	})

	errs := errors.L()
	for _, subBlock := range block.Blocks {
		switch subBlock.Type {
		case "lets":
			errs.AppendWrap(ErrTerramateSchema, letsConfig.mergeBlocks(ast.Blocks{subBlock}))
		case "assert":
			assertCfg, err := parseAssertConfig(subBlock)
			if err != nil {
				errs.Append(err)
				continue
			}
			asserts = append(asserts, assertCfg)
		default:
			// already validated but sanity checks...
			panic(errors.E(errors.ErrInternal, "unexpected block type %s", subBlock.Type))
		}
	}

	context := "stack"
	if contextAttr, ok := block.Body.Attributes["context"]; ok {
		context = hcl.ExprAsKeyword(contextAttr.Expr)
		if context != "stack" && context != "root" {
			errs.Append(errors.E(contextAttr.Expr.Range(),
				"generate_file.context supported values are \"stack\" and \"root\""+
					" but given %q", context))
		}
	}

	mergedLets := ast.MergedLabelBlocks{}
	for labelType, mergedBlock := range letsConfig.MergedLabelBlocks {
		if labelType.Type == "lets" {
			mergedLets[labelType] = mergedBlock

			errs.AppendWrap(ErrTerramateSchema, validateLets(mergedBlock))
		}
	}

	if err := errs.AsError(); err != nil {
		return GenFileBlock{}, err
	}

	lets, ok := mergedLets[ast.NewEmptyLabelBlockType("lets")]
	if !ok {
		lets = ast.NewMergedBlock("lets", []string{})
	}

	return GenFileBlock{
		Range:     block.Range,
		Label:     block.Labels[0],
		Lets:      lets,
		Asserts:   asserts,
		Content:   block.Body.Attributes["content"],
		Condition: block.Body.Attributes["condition"],
		Context:   context,
	}, nil
}

func validateImportBlock(block *ast.Block) error {
	errs := errors.L()
	if len(block.Labels) != 0 {
		errs.Append(errors.E(ErrTerramateSchema, block.LabelRanges(),
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
			{
				Type:       "assert",
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

	return errs.AsError()
}

func validateLets(block *ast.MergedBlock) error {
	if block.Type != "lets" {
		return errors.E(block.RawOrigins[0].TypeRange,
			"unexpected block type %q", block.Type)
	}
	errs := errors.L()
	for _, subBlock := range block.Blocks {
		for _, raw := range subBlock.RawOrigins {
			errs.Append(validateMap(raw))
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
			{
				Name:     "context",
				Required: false,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "lets",
				LabelNames: []string{},
			},
			{
				Type:       "assert",
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
		return nil, errors.E("value must be a list(string), got %q",
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
			block.Block.LabelRanges[i],
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

func parseAssertConfig(assert *ast.Block) (AssertConfig, error) {
	cfg := AssertConfig{}
	errs := errors.L()

	cfg.Range = assert.Range

	errs.Append(checkNoLabels(assert))
	errs.Append(checkHasSubBlocks(assert))

	for _, attr := range assert.Attributes {
		switch attr.Name {
		case "assertion":
			cfg.Assertion = attr.Expr
		case "message":
			cfg.Message = attr.Expr
		case "warning":
			cfg.Warning = attr.Expr
		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", assert.Type, attr.Name,
			))
		}
	}

	if cfg.Assertion == nil {
		errs.Append(errors.E(ErrTerramateSchema, assert.Range,
			"assert.assertion is required"))
	}

	if cfg.Message == nil {
		errs.Append(errors.E(ErrTerramateSchema, assert.Range,
			"assert.message is required"))
	}

	if err := errs.AsError(); err != nil {
		return AssertConfig{}, err
	}

	return cfg, nil
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

	gitBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("git")]
	if ok {
		logger.Trace().Msg("Type is 'git'")

		cfg.Git = NewGitConfig()

		logger.Trace().Msg("Parse git config.")

		errs.Append(parseGitConfig(cfg.Git, gitBlock))
	}

	runBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("run")]
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

	block, ok := runBlock.Blocks[ast.NewEmptyLabelBlockType("env")]
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
		case "assert":
			logger.Trace().Msg("found assert block")
			assertCfg, err := parseAssertConfig(block)
			if err != nil {
				errs.Append(err)
				continue
			}
			config.Asserts = append(config.Asserts, assertCfg)

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

			genhcl, err := parseGenerateHCLBlock(block)
			errs.Append(err)
			if err == nil {
				config.Generate.HCLs = append(config.Generate.HCLs, genhcl)
			}

		case "generate_file":
			logger.Trace().Msg("Found \"generate_file\" block")

			genfile, err := parseGenerateFileBlock(block)
			errs.Append(err)
			if err == nil {
				config.Generate.Files = append(config.Generate.Files, genfile)
			}
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

	globals := ast.MergedLabelBlocks{}
	for labelType, mergedBlock := range rawconfig.MergedLabelBlocks {
		if labelType.Type == "globals" {
			globals[labelType] = mergedBlock

			errs.AppendWrap(ErrTerramateSchema, validateGlobals(mergedBlock))
		}
	}

	config.Globals = globals

	if foundstack {
		logger.Debug().Msg("Parsing stack cfg.")

		if config.Stack != nil {
			errs.Append(errors.E(errKind, stackblock.DefRange(),
				"duplicated stack blocks across configs"))
		}

		config.Stack, err = p.parseStack(stackblock)
		if err != nil {
			errs.AppendWrap(errKind, err)
		}
	}

	if err := errs.AsError(); err != nil {
		return Config{}, err
	}

	config.Imported = p.Imported

	return config, nil
}

func (p *TerramateParser) checkConfigSanity(_ Config) error {
	logger := log.With().
		Str("action", "TerramateParser.checkConfigSanity()").
		Logger()

	rawconfig := p.Imported.Copy()
	_ = rawconfig.Merge(p.Config)

	errs := errors.L()
	tmblock := rawconfig.MergedBlocks["terramate"]
	if tmblock != nil && p.dir != p.rootdir {
		for _, raworigin := range tmblock.RawOrigins {
			if filepath.Dir(raworigin.Range.HostPath()) != p.dir {
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

func validateGlobals(block *ast.MergedBlock) error {
	errs := errors.L()
	if block.Type != "globals" {
		return errors.E(ErrTerramateSchema,
			block.RawOrigins[0].TypeRange, "unexpected block type %q", block.Type)
	}
	errs.Append(block.ValidateSubBlocks("map"))
	for _, raw := range block.RawOrigins {
		for _, subBlock := range raw.Blocks {
			errs.Append(validateMap(subBlock))
		}
	}
	return errs.AsError()
}

func validateMap(block *ast.Block) (err error) {
	if block.Type != "map" {
		return errors.E(block.TypeRange,
			"unexpected block type %s", block.Type)
	}
	if len(block.Labels) == 0 {
		return errors.E(block.LabelRanges(),
			"map block requires a label")
	}
	_, ok := block.Attributes["for_each"]
	if !ok {
		return errors.E(block.Block.TypeRange,
			"map.for_each attribute is required")
	}
	_, ok = block.Attributes["key"]
	if !ok {
		return errors.E(block.TypeRange, "map.key is required")
	}
	_, hasValueAttr := block.Attributes["value"]
	hasValueBlock := false
	for _, subBlock := range block.Blocks {
		if hasValueBlock {
			return errors.E(block.TypeRange,
				"multiple map.value block declared")
		}
		if subBlock.Type != "value" {
			return errors.E(
				subBlock.TypeRange,
				"unrecognized block %s inside map block", subBlock.Type,
			)
		}
		for _, valueSubBlock := range subBlock.Blocks {
			err := validateMap(valueSubBlock)
			if err != nil {
				return err
			}
		}
		hasValueBlock = true
	}

	if hasValueAttr && hasValueBlock {
		return errors.E(block.TypeRange,
			"value attribute conflicts with value block")
	}
	if !hasValueAttr && !hasValueBlock {
		return errors.E(block.TypeRange,
			"either a value attribute or a value block is required")
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
	var foundReqVersion, foundAllowPrereleases bool
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
			if foundReqVersion {
				errs.Append(errors.E(errKind, attr.NameRange,
					"duplicated attribute"))
			}
			foundReqVersion = true
			tm.RequiredVersion = value.AsString()

		case "required_version_allow_prereleases":
			logger.Trace().Msg("Parsing  attribute 'required_version_allow_prereleases'.")

			if value.Type() != cty.Bool {
				errs.Append(errors.E(errKind, attr.Expr.Range(),
					"attribute is not a bool"))

				continue
			}

			if foundAllowPrereleases {
				errs.Append(errors.E(errKind, attr.NameRange,
					"duplicated attribute"))
			}

			foundAllowPrereleases = true
			tm.RequiredVersionAllowPreReleases = value.True()

		default:
			errs.Append(errors.E(errKind, attr.NameRange,
				"unsupported attribute %q", attr.Name))
		}
	}

	errs.AppendWrap(ErrTerramateSchema, block.ValidateSubBlocks("config"))

	logger.Trace().Msg("Parse terramate sub blocks")

	configBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("config")]
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

func hclAttrErr(attr *hcl.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrTerramateSchema, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}

func attrErr(attr ast.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrTerramateSchema, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}
