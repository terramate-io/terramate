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

	"github.com/gobwas/glob"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclparse"
	"github.com/terramate-io/hcl/v2/hclsyntax"
	"github.com/terramate-io/terramate/cloud"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/safeguard"
	"github.com/terramate-io/terramate/stdlib"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/exp/slices"
)

// Errors returned during the HCL parsing.
const (
	ErrHCLSyntax         errors.Kind = "HCL syntax error"
	ErrTerramateSchema   errors.Kind = "terramate schema error"
	ErrUnrecognizedBlock errors.Kind = "terramate schema error: unrecognized block"
	ErrImport            errors.Kind = "import error"
)

// OptionalCheck is a bool that can also have no configured value.
type OptionalCheck int

const (
	// CheckIsUnset means no value was specified.
	CheckIsUnset OptionalCheck = iota
	// CheckIsFalse means the check is disabled.
	CheckIsFalse
	// CheckIsTrue means the check is enabled.
	CheckIsTrue
)

// ValueOr returns if an OptionalCheck is enabled, or the given default if its unset.
func (v OptionalCheck) ValueOr(def bool) bool {
	if v == CheckIsUnset {
		return def
	}
	return v == CheckIsTrue
}

// ToOptionalCheck creates an OptionalCheck value from a bool.
func ToOptionalCheck(v bool) OptionalCheck {
	if v {
		return CheckIsTrue
	}
	return CheckIsFalse
}

// Config represents a Terramate configuration.
type Config struct {
	Terramate       *Terramate
	Stack           *Stack
	Globals         ast.MergedLabelBlocks
	Vendor          *VendorConfig
	Asserts         []AssertConfig
	Generate        GenerateConfig
	Scripts         []*Script
	SharingBackends SharingBackends
	Inputs          Inputs
	Outputs         Outputs

	Imported RawConfig

	// External are parsed configuration from library clients.
	External any

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

// StackFilterConfig represents Terramate stack_filter configuration block.
type StackFilterConfig struct {
	ProjectPaths    []glob.Glob
	RepositoryPaths []glob.Glob
}

// SharingBackendType is the type of the sharing backend.
type SharingBackendType int

// SharingBackends is a list of SharingBackend blocks.
type SharingBackends []SharingBackend

// Inputs is a list of Input blocks.
type Inputs []Input

// Outputs is a list of Output blocks.
type Outputs []Output

// SharingBackend holds the parsed values for the `sharing_backend` block.
type SharingBackend struct {
	Name     string
	Type     SharingBackendType
	Command  []string
	Filename string
}

// Input holds the parsed values for the `input` block.
type Input struct {
	info.Range
	Name        string
	Backend     hcl.Expression
	FromStackID hcl.Expression
	Value       hcl.Expression
	Sensitive   hcl.Expression
	Mock        hcl.Expression
}

// Output holds the parsed value for the `output` block.
type Output struct {
	info.Range
	Name        string
	Backend     hcl.Expression
	Description hcl.Expression
	Value       hcl.Expression
	Sensitive   hcl.Expression
}

// These are the valid sharing_backend types.
const (
	TerraformSharingBackend SharingBackendType = iota + 1
)

func (st SharingBackendType) String() string {
	switch st {
	case TerraformSharingBackend:
		return "terraform"
	}
	return "<unknown>"
}

// MatchAnyGlob is a helper function to test if s matches any of the given patterns.
func MatchAnyGlob(globs []glob.Glob, s string) bool {
	for _, g := range globs {
		if g.Match(s) {
			return true
		}
	}
	return false
}

// RunConfig represents Terramate run configuration.
type RunConfig struct {
	// CheckGenCode enables generated code is up-to-date check on run.
	CheckGenCode bool

	// Env contains environment definitions for run.
	Env *RunEnv
}

// OrderOfExecutionConfig represents the order_of_execution block.
type OrderOfExecutionConfig struct {
	Nested *bool // If filesystem order is enabled.
}

// RunEnv represents Terramate run environment.
type RunEnv struct {
	// Attributes is the collection of attribute definitions within the env block.
	Attributes ast.Attributes
}

// GitConfig represents Terramate Git configuration.
type GitConfig struct {
	// DefaultBranch is the default branch.
	DefaultBranch string

	// DefaultRemote is the default remote.
	DefaultRemote string

	// CheckUntracked enables untracked files checking.
	CheckUntracked bool

	// CheckUncommitted enables uncommitted files checking.
	CheckUncommitted bool

	// CheckRemote enables checking if local default branch is updated with remote.
	CheckRemote OptionalCheck
}

// ChangeDetectionConfig is the `terramate.config.change_detection` config.
type ChangeDetectionConfig struct {
	Terragrunt *TerragruntChangeDetectionConfig
	Git        *GitChangeDetectionConfig
}

// GitChangeDetectionConfig is the `terramate.config.change_detection.git` config.
type GitChangeDetectionConfig struct {
	Untracked   *bool
	Uncommitted *bool
}

// TerragruntChangeDetectionConfig is the `terramate.config.change_detection.terragrunt` config.
type TerragruntChangeDetectionConfig struct {
	Enabled TerragruntChangeDetectionEnabledOption
}

// TerragruntChangeDetectionEnabledOption is the change detection options for enabling Terragrunt.
type TerragruntChangeDetectionEnabledOption int

// Terragrunt Enabling options.
const (
	TerragruntAutoOption TerragruntChangeDetectionEnabledOption = iota
	TerragruntOffOption
	TerragruntForceOption
)

// GenerateRootConfig represents the AST node for the `terramate.config.generate` block.
type GenerateRootConfig struct {
	HCLMagicHeaderCommentStyle *string
}

// CloudConfig represents Terramate cloud configuration.
type CloudConfig struct {
	// Organization is the name of the cloud organization
	Organization string

	Targets *TargetsConfig

	Location cloud.Region
}

// TargetsConfig represents Terramate targets configuration.
type TargetsConfig struct {
	Enabled bool
}

// TelemetryConfig represents Terramate telemetry configuration.
type TelemetryConfig struct {
	Enabled *bool
}

// RootConfig represents the root config block of a Terramate configuration.
type RootConfig struct {
	Git               *GitConfig
	Generate          *GenerateRootConfig
	ChangeDetection   *ChangeDetectionConfig
	Run               *RunConfig
	OrderOfExecution  *OrderOfExecutionConfig
	Cloud             *CloudConfig
	Experiments       []string
	DisableSafeguards safeguard.Keywords
	Telemetry         *TelemetryConfig
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
	// Dir where the block is declared.
	Dir project.Path

	// Range is the range of the entire block definition.
	Range info.Range
	// Label of the block.
	Label string
	// Lets is a block of local variables.
	Lets *ast.MergedBlock
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Represents all stack_filter blocks
	StackFilters []StackFilterConfig
	// Content block.
	Content *hcl.Block
	// Asserts represents all assert blocks
	Asserts []AssertConfig

	// Inherit tells if the block is inherited in child directories.
	Inherit *hclsyntax.Attribute

	// IsImplicitBlock tells if the block is implicit (does not have a real generate_hcl block).
	// This is the case for the "tmgen" feature.
	IsImplicitBlock bool
}

// GenFileBlock represents a parsed generate_file block
type GenFileBlock struct {
	// Dir where the block is declared.
	Dir project.Path

	// Range is the range of the entire block definition.
	Range info.Range
	// Label of the block
	Label string
	// Lets is a block of local variables.
	Lets *ast.MergedBlock
	// Condition attribute of the block, if any.
	Condition *hclsyntax.Attribute
	// Represents all stack_filter blocks
	StackFilters []StackFilterConfig
	// Content attribute of the block
	Content *hclsyntax.Attribute
	// Context of the generation (stack by default).
	Context string
	// Asserts represents all assert blocks
	Asserts []AssertConfig

	// Inherit tells if the block is inherited in child directories.
	Inherit *hclsyntax.Attribute
}

// Evaluator represents a Terramate evaluator
type Evaluator interface {
	// Eval evaluates the given expression returning a value.
	Eval(hcl.Expression) (cty.Value, error)

	// PartialEval partially evaluates the given expression returning the
	// tokens that form the result of the partial evaluation. Any unknown
	// namespace access are ignored and left as is, while known namespaces
	// are substituted by its value.
	// If any unknowns are found, the method returns hasUnknowns as true.
	PartialEval(hcl.Expression) (expr hcl.Expression, hasUnknowns bool, err error)

	// SetNamespace adds a new namespace, replacing any with the same name.
	SetNamespace(name string, values map[string]cty.Value)

	// DeleteNamespace deletes a namespace.
	DeleteNamespace(name string)
}

type parserState int

const (
	initialState parserState = iota
	syntaxParsedState
	importsAppliedState
	mergedConfigState
	schemaParsedState
	schemaValidatedState
)

// TerramateParser is an HCL parser tailored for Terramate configuration schema.
// As the Terramate configuration can span multiple files in the same directory,
// this API allows you to define the exact set of files (and contents) that are
// going to be included in the final configuration.
type TerramateParser struct {
	Config   RawConfig
	Imported RawConfig

	rootdir string
	dir     string

	// options
	experiments               []string
	strict                    bool
	unmergedBlockHandlers     map[string]UnmergedBlockHandler
	mergedBlockHandlers       map[string]MergedBlockHandler
	mergedLabelsBlockHandlers map[string]MergedLabelsBlockHandler
	uniqueBlockHandlers       map[string]UniqueBlockHandler

	files     map[string][]byte // path=content
	hclparser *hclparse.Parser
	evalctx   *eval.Context

	// parsedFiles stores a map of all parsed files
	parsedFiles map[string]parsedFile

	ParsedConfig Config

	state parserState
}

// Option is a function that can be used to configure a TerramateParser.
type Option func(*TerramateParser)

// UnmergedBlockHandler specifies how the block should be parsed.
type UnmergedBlockHandler interface {
	// Name is the block name (e.g. "stack").
	Name() string

	// Parse parses the block.
	Parse(*TerramateParser, *ast.Block) error
}

// MergedBlockHandler specifies how a merged block should be parsed.
type MergedBlockHandler interface {
	// Name is the block name (e.g. "terramate").
	Name() string

	// Parse parses the block.
	Parse(*TerramateParser, *ast.MergedBlock) error

	// Validate postconditions after parsing.
	Validate(*TerramateParser) error
}

// MergedLabelsBlockHandler specifies how a merged block with labels should be parsed.
type MergedLabelsBlockHandler interface {
	// Name is the block name (e.g. "globals").
	Name() string

	// Parse parses the block.
	Parse(*TerramateParser, ast.LabelBlockType, *ast.MergedBlock) error

	// Validate postconditions after parsing.
	Validate(*TerramateParser) error
}

// UniqueBlockHandler specifies how a unique block should be parsed.
type UniqueBlockHandler interface {
	// Name is the block name (e.g. "vendor").
	Name() string

	// Parse parses the block.
	Parse(*TerramateParser, *ast.Block) error
}

// NewGitConfig creates a git configuration with proper default values.
func NewGitConfig() *GitConfig {
	return &GitConfig{
		CheckUntracked:   true,
		CheckUncommitted: true,
	}
}

// NewRunConfig creates a new run configuration.
func NewRunConfig() *RunConfig {
	return &RunConfig{
		CheckGenCode: true,
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

// NewTerramateParser creates a Terramate parser for the directory dir inside the root directory.
// The parser creates sub-parsers for parsing imports but keeps a list of all
// parsed files of all sub-parsers for detecting cycles and import duplications.
// The subparsers inherits this parser options.
func NewTerramateParser(rootdir string, dir string, opts ...Option) (*TerramateParser, error) {
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

	p := &TerramateParser{
		Config:   NewTopLevelRawConfig(),
		Imported: NewTopLevelRawConfig(),
		rootdir:  rootdir,
		dir:      dir,

		// state
		files:                     map[string][]byte{},
		hclparser:                 hclparse.NewParser(),
		parsedFiles:               make(map[string]parsedFile),
		unmergedBlockHandlers:     map[string]UnmergedBlockHandler{},
		mergedBlockHandlers:       map[string]MergedBlockHandler{},
		mergedLabelsBlockHandlers: map[string]MergedLabelsBlockHandler{},
		uniqueBlockHandlers:       map[string]UniqueBlockHandler{},
		state:                     initialState,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.evalctx = eval.NewContext(stdlib.Functions(dir, p.experiments))

	if len(p.unmergedBlockHandlers) == 0 {
		for _, spec := range DefaultUnmergedBlockParsers() {
			p.addUnmergedBlockHandler(spec)
		}
	}
	if len(p.mergedBlockHandlers) == 0 {
		for _, spec := range DefaultMergedBlockHandlers() {
			p.addMergedBlockHandler(spec)
		}
	}
	if len(p.mergedLabelsBlockHandlers) == 0 {
		for _, spec := range DefaultMergedLabelsBlockHandlers() {
			p.addMergedLabelsBlockHandler(spec)
		}
	}
	if len(p.uniqueBlockHandlers) == 0 {
		for _, spec := range DefaultUniqueBlockHandlers() {
			p.addUniqueBlockHandler(spec)
		}
	}
	return p, nil
}

func (p *TerramateParser) addUnmergedBlockHandler(spec UnmergedBlockHandlerConstructor) {
	p.unmergedBlockHandlers[spec().Name()] = spec()
	p.Config.dupeHandlers[spec().Name()] = (*RawConfig).addBlock
	p.Imported.dupeHandlers[spec().Name()] = (*RawConfig).addBlock
}

func (p *TerramateParser) addMergedBlockHandler(spec MergedBlockHandlerConstructor) {
	p.mergedBlockHandlers[spec().Name()] = spec()
	p.Config.dupeHandlers[spec().Name()] = (*RawConfig).mergeBlock
	p.Imported.dupeHandlers[spec().Name()] = (*RawConfig).mergeBlock
}

func (p *TerramateParser) addMergedLabelsBlockHandler(spec MergedLabelsBlockHandlerConstructor) {
	p.mergedLabelsBlockHandlers[spec().Name()] = spec()
	p.Config.dupeHandlers[spec().Name()] = (*RawConfig).mergeLabeledBlock
	p.Imported.dupeHandlers[spec().Name()] = (*RawConfig).mergeLabeledBlock
}

func (p *TerramateParser) addUniqueBlockHandler(spec UniqueBlockHandlerConstructor) {
	p.uniqueBlockHandlers[spec().Name()] = spec()
	p.Config.dupeHandlers[spec().Name()] = (*RawConfig).addUniqueBlock
	p.Imported.dupeHandlers[spec().Name()] = (*RawConfig).addUniqueBlock
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
	res, err := fs.ListTerramateFiles(dir)
	if err != nil {
		return errors.E(err, "adding directory to terramate parser")
	}

	for _, filename := range res.TmFiles {
		path := filepath.Join(dir, filename)

		data, err := os.ReadFile(path)
		if err != nil {
			return errors.E(err, "reading config file %q", path)
		}

		if err := p.AddFileContent(path, data); err != nil {
			return err
		}
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
func (p *TerramateParser) ParseConfig() (*Config, error) {
	errs := errors.L()
	errs.Append(p.ParseHCL())

	switch p.state {
	case mergedConfigState:
		// TODO(i4k): don't validate schema here.
		// Changing this requires changes to the editor extensions / linters / etc.
		var err error
		_, err = p.parseTerramateSchema()
		errs.Append(err)
		fallthrough
	case schemaParsedState:
		if err := errs.AsError(); err == nil {
			errs.Append(p.checkConfigSanity())
		}
	case schemaValidatedState:
		return &p.ParsedConfig, nil
	default:
		panic(errors.E(errors.ErrInternal, "invalid parser state: %d: report this bug", p.state))
	}

	if err := errs.AsError(); err != nil {
		return &Config{}, err
	}
	return &p.ParsedConfig, nil
}

// ParseHCL does the syntax parsing, applying imports and merging of configurations but do not
// validate if the HCL schema is a valid Terramate configuration.
func (p *TerramateParser) ParseHCL() error {
	// This function is a NOOP if state > importsAppliedState
	errs := errors.L()
	switch p.state {
	case initialState:
		errs.Append(p.parseSyntax())
		fallthrough
	case syntaxParsedState:
		errs.Append(p.applyImports())
		fallthrough
	case importsAppliedState:
		errs.Append(p.mergeConfig())
	}
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

// mergeConfigInto merges all ASTs (from all files) into the given RawConfig.
// This is useful if you want to merge the config but without doing the other steps
// like applying imports or parsing the schema (and without moving the parser state).
// One such example is for checking if the config is a root config because the imports
// could be absolute to the root directory, which is not known at the time of the check.
// For reference, see this issue: https://github.com/terramate-io/terramate/issues/515
// and the test TestBug515
func (p *TerramateParser) mergeConfigInto(cfg *RawConfig) error {
	errs := errors.L()
	bodies := p.ParsedBodies()
	for _, origin := range p.sortedParsedFilenames() {
		body := bodies[origin]
		errs.Append(cfg.mergeAttrs(ast.NewAttributes(p.rootdir, ast.AsHCLAttributes(body.Attributes))))
		errs.Append(cfg.mergeBlocks(ast.NewBlocks(p.rootdir, body.Blocks)))
	}
	return errs.AsError()
}

func (p *TerramateParser) mergeConfig() error {
	if p.state >= mergedConfigState {
		return nil
	}
	defer func() {
		p.state = mergedConfigState
	}()
	return p.mergeConfigInto(&p.Config)
}

func (p *TerramateParser) parseSyntax() error {
	if p.state >= syntaxParsedState {
		return nil
	}
	defer func() {
		p.state = syntaxParsedState
	}()
	errs := errors.L()
	for _, name := range p.sortedFilenames() {
		if _, ok := p.parsedFiles[name]; ok {
			continue
		}
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
	if p.state >= importsAppliedState {
		return nil
	}
	defer func() {
		p.state = importsAppliedState
	}()
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
		err = importParser.ParseHCL()
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

// IsRootConfig tells if the Config is a root configuration.
func (c Config) IsRootConfig() bool {
	return c.Terramate != nil && c.Terramate.RequiredVersion != ""
}

// HasRunEnv returns true if the config has a terramate.config.run.env block defined
func (c Config) HasRunEnv() bool {
	return c.Terramate != nil &&
		c.Terramate.Config != nil &&
		c.Terramate.Config.Run != nil &&
		c.Terramate.Config.Run.Env != nil
}

// Experiments returns the config enabled experiments, if any.
func (c Config) Experiments() []string {
	if c.Terramate != nil &&
		c.Terramate.Config != nil {
		return c.Terramate.Config.Experiments
	}
	return []string{}
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
func ParseDir(root string, dir string, opts ...Option) (*Config, error) {
	p, err := NewTerramateParser(root, dir, opts...)
	if err != nil {
		return &Config{}, err
	}
	err = p.AddDir(dir)
	if err != nil {
		return &Config{}, errors.E("adding files to parser", err)
	}
	return p.ParseConfig()
}

// IsRootConfig parses rootdir and tells if it contains a root config or not.
// Note: after identifying this is the config you need, then call ParseConfig()
// to retrieve the full config.
func (p *TerramateParser) IsRootConfig() (bool, error) {
	errs := errors.L()
	cfg := p.Config.Copy() // copy all merge handlers
	errs.Append(p.parseSyntax())
	errs.Append(p.mergeConfigInto(&cfg))
	if err := errs.AsError(); err != nil {
		return false, err
	}
	if p.state != syntaxParsedState {
		panic(errors.E(errors.ErrInternal, "mergeConfigInto() advanced the parser state to %d", p.state))
	}
	terramate := cfg.MergedBlocks["terramate"]
	if terramate == nil {
		return false, nil
	}
	if _, ok := terramate.Attributes["required_version"]; ok {
		return ok, nil
	}
	return false, nil
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
			{
				Name:     "inherit",
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
			{
				Type:       "stack_filter",
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
	errs := errors.L()
	for _, subBlock := range block.Blocks {
		for _, raw := range subBlock.RawOrigins {
			if raw.Type != "map" {
				return errors.E(ErrUnrecognizedBlock, "the block %s is not expected here", raw.Type)
			}
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
				Name:     "inherit",
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
			{
				Type:       "stack_filter",
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

func assignSet(attr *hcl.Attribute, target *[]string, val cty.Value) error {
	if val.IsNull() {
		return nil
	}

	// as the parser is schemaless it only creates tuples (lists of arbitrary types).
	// we have to check the elements themselves.
	if !val.Type().IsTupleType() && !val.Type().IsListType() {
		return errors.E(ErrTerramateSchema, attr.Expr.Range(),
			"field %q must be a set(string) but found a %q", attr.Name, val.Type().FriendlyName())
	}

	errs := errors.L()
	var elems []string
	values := map[string]struct{}{}
	iterator := val.ElementIterator()
	index := -1
	for iterator.Next() {
		index++
		_, elem := iterator.Element()

		if elem.Type() != cty.String {
			errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(),
				"field %q must be a set(string) but element %d has type %q",
				attr.Name, index, elem.Type().FriendlyName()))

			continue
		}

		str := elem.AsString()
		if _, ok := values[str]; ok {
			errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(),
				"duplicated entry %q in the index %d of field %q of type set(string)",
				str, index, attr.Name))

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

func parseStackFilterConfig(block *ast.Block) (StackFilterConfig, error) {
	cfg := StackFilterConfig{}
	errs := errors.L()

	errs.Append(checkNoLabels(block))
	errs.Append(checkHasSubBlocks(block))

	for _, attr := range block.Attributes {
		switch attr.Name {
		case "project_paths":
			var err error
			cfg.ProjectPaths, err = parseStackFilterAttr(attr)
			errs.Append(err)

		case "repository_paths":
			var err error
			cfg.RepositoryPaths, err = parseStackFilterAttr(attr)
			errs.Append(err)

		default:
			errs.Append(errors.E(ErrTerramateSchema, attr.NameRange,
				"unrecognized attribute %s.%s", block.Type, attr.Name,
			))
		}
	}

	if err := errs.AsError(); err != nil {
		return StackFilterConfig{}, err
	}

	return cfg, nil
}

func parseStackFilterAttr(attr ast.Attribute) ([]glob.Glob, error) {
	attrVal, hclerr := attr.Expr.Value(nil)
	if hclerr != nil {
		return nil, errors.E(ErrTerramateSchema, hclerr, attr.NameRange, "evaluating %s", attr.Name)
	}

	r, err := ValueAsStringList(attrVal)
	if err != nil {
		return nil, errors.E(ErrTerramateSchema, err, attr.NameRange)
	}

	if len(r) == 0 {
		return nil, errors.E(ErrTerramateSchema, attr.NameRange, "%s must not be empty", attr.Name)
	}

	var globs []glob.Glob

	for _, s := range r {
		// Add ** prefix as default.
		if !strings.HasPrefix(s, "*") && !strings.HasPrefix(s, "/") {
			s = "**/" + s
		}

		for _, escaped := range []string{`\`, `{`, `}`} {
			s = strings.ReplaceAll(s, escaped, `\`+escaped)
		}

		g, err := glob.Compile(s, '/')
		if err != nil {
			return nil, errors.E(ErrTerramateSchema, err, attr.NameRange,
				"compiling match pattern for %s", attr.Name)
		}

		globs = append(globs, g)
	}

	return globs, nil

}

// hasExperimentalFeature returns true if the config has the provided experimental feature enabled.
func (p *TerramateParser) hasExperimentalFeature(feature string) bool {
	return slices.Contains(p.experiments, feature)
}

func (p *TerramateParser) parseRootConfig(cfg *RootConfig, block *ast.MergedBlock) error {
	errs := errors.L()

	for _, attr := range block.Attributes.SortedList() {
		switch attr.Name {
		default:
			errs.Append(errors.E(attr.NameRange,
				"unrecognized attribute terramate.config.%s", attr.Name,
			))
			continue
		case "experiments":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				errs.Append(errors.E(diags, attr.Expr.Range(),
					"evaluating terramate.config.experiments attribute"))
				continue
			}

			if err := assignSet(attr.Attribute, &cfg.Experiments, val); err != nil {
				errs.Append(err)
				continue
			}
			p.experiments = cfg.Experiments
		case "disable_safeguards":
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				errs.Append(errors.E(diags, attr.Expr.Range(),
					"evaluating terramate.config.disable_safeguards attribute"))
				continue
			}

			keywordStrings := []string{}
			if err := assignSet(attr.Attribute, &keywordStrings, val); err != nil {
				errs.Append(err)
				continue
			}
			var err error
			cfg.DisableSafeguards, err = safeguard.FromStrings(keywordStrings)
			if err != nil {
				errs.Append(errors.E(ErrTerramateSchema, attr.Expr.Range(), err))
			}
		}
	}

	errs.AppendWrap(ErrTerramateSchema, block.ValidateSubBlocks(
		"git",
		"generate",
		"change_detection",
		"run",
		"order_of_execution",
		"cloud",
		"targets",
		"telemetry",
	))

	gitBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("git")]
	if ok {
		errs.Append(parseGitConfig(cfg, gitBlock))
	}

	runBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("run")]
	if ok {
		errs.Append(parseRunConfig(cfg, runBlock))
	}

	orderExecBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("order_of_execution")]
	if ok {
		cfg.OrderOfExecution = &OrderOfExecutionConfig{}
		errs.Append(parseOrderOfExecutionConfig(cfg.OrderOfExecution, orderExecBlock))
	}

	cloudBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("cloud")]
	if ok {
		cfg.Cloud = &CloudConfig{}

		errs.Append(parseCloudConfig(cfg.Cloud, cloudBlock))
	}

	generateBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("generate")]
	if ok {
		cfg.Generate = &GenerateRootConfig{}

		errs.Append(parseGenerateRootConfig(cfg.Generate, generateBlock))
	}

	changeDetectionBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("change_detection")]
	if ok {
		cfg.ChangeDetection = &ChangeDetectionConfig{}
		errs.Append(parseChangeDetectionConfig(cfg.ChangeDetection, changeDetectionBlock))
	}

	telemetryBlock, ok := block.Blocks[ast.NewEmptyLabelBlockType("telemetry")]
	if ok {
		cfg.Telemetry = &TelemetryConfig{}
		errs.Append(parseTelemetryConfigBlock(cfg.Telemetry, telemetryBlock))
	}

	return errs.AsError()
}

func parseRunConfig(cfg *RootConfig, runBlock *ast.MergedBlock) error {
	cfg.Run = &RunConfig{
		CheckGenCode: true,
	}
	runCfg := cfg.Run
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
			if len(cfg.DisableSafeguards) > 0 {
				errs.AppendWrap(ErrTerramateSchema, attrErr(attr,
					"terramate.config.run.check_gen_code conflicts with terramate.config.disable_safeguards",
				))
				continue
			}
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

func parseOrderOfExecutionConfig(cfg *OrderOfExecutionConfig, orderExecBlock *ast.MergedBlock) error {
	errs := errors.L()
	for _, attr := range orderExecBlock.Attributes {
		value, err := attr.Expr.Value(nil)
		if err != nil {
			errs.Append(errors.E(err, "failed to evaluate terramate.config.order_of_execution.%s attribute", attr.Name))
			continue
		}

		switch attr.Name {
		case "nested":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.order_of_execution.nested is not a bool but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			t := value.True()
			cfg.Nested = &t
		default:
			errs.Append(errors.E(attr.NameRange,
				"unrecognized attribute terramate.config.order_of_execution.%s",
				attr.Name,
			))
		}
	}
	return errs.AsError()
}

func parseGenerateRootConfig(cfg *GenerateRootConfig, generateBlock *ast.MergedBlock) error {
	errs := errors.L()

	errs.AppendWrap(ErrTerramateSchema, generateBlock.ValidateSubBlocks())

	for _, attr := range generateBlock.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.generate.%s attribute", attr.Name,
			))
			continue
		}

		switch attr.Name {
		case "hcl_magic_header_comment_style":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.generate.hcl_magic_header_comment_style is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			str := value.AsString()
			if str != "//" && str != "#" {
				errs.Append(attrErr(attr,
					"terramate.config.generate.hcl_magic_header_comment_style must be either `//` or `#` but %q was given",
					str,
				))
				continue
			}

			cfg.HCLMagicHeaderCommentStyle = &str

		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.generate.%s",
				attr.Name,
			))
		}
	}
	return errs.AsError()
}

func parseChangeDetectionConfig(cfg *ChangeDetectionConfig, changeDetectionBlock *ast.MergedBlock) error {
	err := changeDetectionBlock.ValidateSubBlocks("terragrunt", "git")
	if err != nil {
		return err
	}
	terragruntBlock, ok := changeDetectionBlock.Blocks[ast.NewEmptyLabelBlockType("terragrunt")]
	if ok {
		cfg.Terragrunt = &TerragruntChangeDetectionConfig{}
		err := parseTerragruntChangeDetectionConfig(cfg.Terragrunt, terragruntBlock)
		if err != nil {
			return err
		}
	}

	gitBlock, ok := changeDetectionBlock.Blocks[ast.NewEmptyLabelBlockType("git")]
	if ok {
		cfg.Git = &GitChangeDetectionConfig{}
		err := parseGitChangeDetectionConfig(cfg.Git, gitBlock)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseTerragruntChangeDetectionConfig(cfg *TerragruntChangeDetectionConfig, terragruntBlock *ast.MergedBlock) error {
	errs := errors.L()
	errs.Append(terragruntBlock.ValidateSubBlocks())

	for _, attr := range terragruntBlock.Attributes {
		switch attr.Name {
		case "enabled":
			value, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				errs.Append(errors.E(diags,
					"failed to evaluate terramate.config.change_detection.terragrunt.%s attribute", attr.Name,
				))
				continue
			}
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.change_detection.terragrunt.enabled is not a string but %q",
					value.Type().FriendlyName(),
				))
				continue
			}

			valStr := value.AsString()
			var opt TerragruntChangeDetectionEnabledOption
			switch valStr {
			case "auto":
				opt = TerragruntAutoOption
			case "off":
				opt = TerragruntOffOption
			case "force":
				opt = TerragruntForceOption
			default:
				errs.Append(attrErr(attr,
					`terramate.config.change_detection.terragrunt.enabled must be either "auto", "off" or "force" but %q was given`,
					valStr,
				))
			}

			cfg.Enabled = opt
		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.change_detection.terragrunt.%s",
				attr.Name,
			))
		}
	}
	return nil
}

func parseTelemetryConfigBlock(cfg *TelemetryConfig, telemetryBlock *ast.MergedBlock) error {
	errs := errors.L()

	for _, attr := range telemetryBlock.Attributes {
		switch attr.Name {
		case "enabled":
			value, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				errs.Append(errors.E(diags,
					"failed to evaluate terramate.config.telemetry.%s attribute", attr.Name,
				))
				continue
			}
			switch value.Type() {
			case cty.String:
				valStr := value.AsString()
				switch valStr {
				case "on":
					v := true
					cfg.Enabled = &v
				case "off":
					v := false
					cfg.Enabled = &v
				default:
					errs.Append(errors.E("unexpected value %q in the `terramate.config.telemetry.%s` attribute", valStr, attr.Name))
				}
			case cty.Bool:
				v := value.True()
				cfg.Enabled = &v
			default:
				errs.Append(errors.E("expected `string` or `bool` but type %s is set in the `terramate.config.telemetry.%s` attribute", attr.Name, value.Type().FriendlyName()))
			}

		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.telemetry.%s",
				attr.Name,
			))
		}
	}
	return nil
}

func parseGitChangeDetectionConfig(cfg *GitChangeDetectionConfig, gitBlock *ast.MergedBlock) error {
	errs := errors.L()
	errs.Append(gitBlock.ValidateSubBlocks())

	handleAttr := func(attr ast.Attribute, option **bool) {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.change_detection.git.%s attribute", attr.Name,
			))
			return
		}
		switch value.Type() {
		case cty.String:
			valStr := value.AsString()
			switch valStr {
			case "on":
				val := true
				*option = &val
			case "off":
				val := false
				*option = &val
			default:
				errs.Append(errors.E("unexpected value %q in the `terramate.config.change_detection.git.%s` attribute", valStr, attr.Name))
			}
		case cty.Bool:
			valBool := value.True()
			*option = &valBool
		default:
			errs.Append(errors.E("expected `string` or `bool` but type %s is set in the `terramate.config.change_detection.git.%s` attribute", attr.Name, value.Type().FriendlyName()))
		}
	}

	for _, attr := range gitBlock.Attributes {
		switch attr.Name {
		case "untracked":
			handleAttr(attr, &cfg.Untracked)
		case "uncommitted":
			handleAttr(attr, &cfg.Uncommitted)
		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.change_detection.git.%s",
				attr.Name,
			))
		}
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

func parseGitConfig(cfg *RootConfig, gitBlock *ast.MergedBlock) error {
	errs := errors.L()

	errs.AppendWrap(ErrTerramateSchema, gitBlock.ValidateSubBlocks())

	cfg.Git = NewGitConfig()
	git := cfg.Git

	for _, attr := range gitBlock.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.%s attribute", attr.Name,
			))
			continue
		}

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

		case "check_untracked":
			if err := checkSafeguardConfigConflict(cfg, attr); err != nil {
				errs.AppendWrap(ErrTerramateSchema, err)
				continue
			}
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_untracked is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}

			git.CheckUntracked = value.True()
		case "check_uncommitted":
			if err := checkSafeguardConfigConflict(cfg, attr); err != nil {
				errs.AppendWrap(ErrTerramateSchema, err)
				continue
			}
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_uncommitted is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			git.CheckUncommitted = value.True()
		case "check_remote":
			if err := checkSafeguardConfigConflict(cfg, attr); err != nil {
				errs.AppendWrap(ErrTerramateSchema, err)
				continue
			}
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.git.check_remote is not a boolean but %q",
					value.Type().FriendlyName(),
				))
				continue
			}
			git.CheckRemote = ToOptionalCheck(value.True())

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

func checkSafeguardConfigConflict(cfg *RootConfig, attr ast.Attribute) error {
	if len(cfg.DisableSafeguards) > 0 {
		return attrErr(attr,
			"terramate.config.git.%s conflicts with terramate.config.disable_safeguards",
			attr.Name,
		)
	}
	return nil
}

func parseCloudConfig(cloudcfg *CloudConfig, cloudBlock *ast.MergedBlock) error {
	errs := errors.L()

	for _, attr := range cloudBlock.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.cloud.%s attribute", attr.Name,
			))
			continue
		}

		switch attr.Name {
		case "organization":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.cloud.organization is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			cloudcfg.Organization = value.AsString()

		case "location":
			if value.Type() != cty.String {
				errs.Append(attrErr(attr,
					"terramate.config.cloud.location is not a string but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			location, err := cloud.ParseRegion(value.AsString())
			if err != nil {
				errs.Append(attrErr(attr,
					"terramate.config.cloud.location is not a valid region (%s) but %q",
					cloud.AvailableRegions(),
					value.AsString(),
				))

				continue
			}

			cloudcfg.Location = location

		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.cloud.%s",
				attr.Name,
			))
		}
	}

	errs.AppendWrap(ErrTerramateSchema, cloudBlock.ValidateSubBlocks("targets"))

	targetsBlock, ok := cloudBlock.Blocks[ast.NewEmptyLabelBlockType("targets")]
	if ok {
		cloudcfg.Targets = &TargetsConfig{}

		errs.Append(parseTargetsConfig(cloudcfg.Targets, targetsBlock))
	}

	return errs.AsError()
}

func parseTargetsConfig(targets *TargetsConfig, targetsBlock *ast.MergedBlock) error {
	errs := errors.L()

	errs.AppendWrap(ErrTerramateSchema, targetsBlock.ValidateSubBlocks())

	for _, attr := range targetsBlock.Attributes.SortedList() {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			errs.Append(errors.E(diags,
				"failed to evaluate terramate.config.cloud.targets.%s attribute", attr.Name,
			))
			continue
		}

		switch attr.Name {
		case "enabled":
			if value.Type() != cty.Bool {
				errs.Append(attrErr(attr,
					"terramate.config.cloud.targets.enabled is not a boolean but %q",
					value.Type().FriendlyName(),
				))

				continue
			}

			targets.Enabled = value.True()

		default:
			errs.Append(errors.E(
				attr.NameRange,
				"unrecognized attribute terramate.config.cloud.targets.%s",
				attr.Name,
			))
		}
	}
	return errs.AsError()
}

func (p *TerramateParser) parseTerramateSchema() (*Config, error) {
	defer func() { p.state = schemaParsedState }()
	p.ParsedConfig = Config{
		absdir: p.dir,
	}

	config := &p.ParsedConfig
	errKind := ErrTerramateSchema
	errs := errors.L()

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

	for _, mergedBlock := range rawconfig.MergedBlocks {
		if spec, ok := p.mergedBlockHandlers[string(mergedBlock.Type)]; ok {
			err := spec.Parse(p, mergedBlock)
			errs.Append(err)
		}
	}

	for _, block := range rawconfig.UnmergedBlocks {
		// unmerged blocks

		if spec := p.unmergedBlockHandlers[block.Type]; spec != nil {
			err = spec.Parse(p, block)
			errs.Append(err)
			continue
		}
	}

	config.Imported = p.Imported

	for _, uniqueBlock := range rawconfig.UniqueBlocks {
		if spec, ok := p.uniqueBlockHandlers[string(uniqueBlock.Type)]; ok {
			err := spec.Parse(p, uniqueBlock)
			errs.Append(err)
		}
	}

	for label, mergedBlock := range rawconfig.MergedLabelBlocks {
		if spec, ok := p.mergedLabelsBlockHandlers[string(label.Type)]; ok {
			err := spec.Parse(p, label, mergedBlock)
			errs.Append(err)
		}
	}

	if err := errs.AsError(); err != nil {
		return &Config{}, err
	}
	return config, nil
}

func (p *TerramateParser) checkConfigSanity() error {
	defer func() { p.state = schemaValidatedState }()
	logger := log.With().
		Str("action", "TerramateParser.checkConfigSanity()").
		Logger()

	rawconfig := p.Imported.Copy()
	_ = rawconfig.Merge(p.Config)

	errs := errors.L()
	tmblock := rawconfig.MergedBlocks["terramate"]
	if tmblock != nil && p.dir != p.rootdir {
		for _, raworigin := range tmblock.RawOrigins {
			for _, attr := range raworigin.Attributes.SortedList() {
				errs.Append(attributeSanityCheckErr(p.dir, "terramate", attr))
			}
			for _, block := range raworigin.Blocks {
				if block.Type == "config" {
					errs.Append(terramateConfigBlockSanityCheck(p.dir, block))
				} else {
					errs.Append(blockSanityCheckErr(p.dir, "terramate", block))
				}
			}
		}
	}
	if p.strict {
		return errs.AsError()
	}
	for _, err := range errs.Errors() {
		logger.Warn().Err(err).Send()
	}

	return p.validateAfterParsing()
}

func (p *TerramateParser) validateAfterParsing() error {
	// some definitions are incremental, so we don't know if a required attribute is missing until the end
	errs := errors.L()

	for _, handler := range p.mergedBlockHandlers {
		errs.Append(handler.Validate(p))
	}
	for _, handler := range p.mergedLabelsBlockHandlers {
		errs.Append(handler.Validate(p))
	}

	return errs.AsError()
}

func terramateConfigRunSanityCheck(parsingDir string, runblock *ast.Block) error {
	errs := errors.L()
	for _, attr := range runblock.Attributes.SortedList() {
		errs.Append(attributeSanityCheckErr(parsingDir, "terramate.config.run", attr))
	}
	for _, block := range runblock.Blocks {
		if block.Type == "env" {
			continue
		}
		errs.Append(blockSanityCheckErr(parsingDir, "terramate.config.run", block))
	}
	return errs.AsError()
}

func terramateConfigBlockSanityCheck(parsingDir string, cfgblock *ast.Block) error {
	errs := errors.L()
	for _, attr := range cfgblock.Attributes.SortedList() {
		errs.Append(attributeSanityCheckErr(parsingDir, "terramate.config", attr))
	}
	for _, block := range cfgblock.Blocks {
		if block.Type == "run" {
			errs.Append(terramateConfigRunSanityCheck(parsingDir, block))
		} else {
			errs.Append(blockSanityCheckErr(parsingDir, "terramate.config", block))
		}
	}
	return errs.AsError()
}

func blockSanityCheckErr(parsingDir string, baseBlockName string, block *ast.Block) error {
	err := errors.E("block %s.%s can only be declared at the project root directory", baseBlockName, block.Type)
	if filepath.Dir(block.Range.HostPath()) != parsingDir {
		err = errors.E(ErrTerramateSchema, err, block.TypeRange,
			"imported from directory %q", parsingDir)
	} else {
		err = errors.E(ErrTerramateSchema, err, block.TypeRange)
	}
	return err
}

func attributeSanityCheckErr(parsingDir string, baseBlockName string, attr ast.Attribute) error {
	err := errors.E("attribute %s.%s can only be declared at the project root directory", baseBlockName, attr.Name)
	if filepath.Dir(attr.Range.HostPath()) != parsingDir {
		err = errors.E(ErrTerramateSchema, err, attr.NameRange,
			"imported from directory %q", parsingDir)
	} else {
		err = errors.E(ErrTerramateSchema, err, attr.NameRange)
	}
	return err
}

func validateMap(block *ast.Block) (err error) {
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
			if valueSubBlock.Type != "map" {
				return errors.E(ErrTerramateSchema, "unexpected block type %s", valueSubBlock.Type)
			}
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

func (p *TerramateParser) evalStringList(expr hcl.Expression, name string) ([]string, error) {
	list, err := p.evalctx.Eval(expr)
	if err != nil {
		return nil, err
	}
	if !list.Type().IsListType() && !list.Type().IsTupleType() {
		return nil, errors.E(`%q must be a list(string) but %q given`, name, list.Type().FriendlyName())
	}
	if list.LengthInt() == 0 {
		return nil, errors.E(`%q must be a non-empty list of strings`, name)
	}
	var command []string
	it := list.ElementIterator()
	index := 0
	for it.Next() {
		_, val := it.Element()
		if !val.Type().Equals(cty.String) {
			return nil, errors.E(`element %d of attribute %s is not a string but %s`, index, name, val.Type().FriendlyName())
		}
		command = append(command, val.AsString())
		index++
	}
	return command, nil
}

// HasSafeguardDisabled checks if the configuration (including the deprecated) has
// the given keyword disabled.
func (r *RootConfig) HasSafeguardDisabled(keyword safeguard.Keyword) bool {
	git := r.Git
	if git == nil {
		git = NewGitConfig()
	}
	run := r.Run
	if run == nil {
		run = NewRunConfig()
	}
	if r.DisableSafeguards.Has("all") {
		return true
	}
	if r.DisableSafeguards.Has("none") {
		return false
	}
	switch keyword {
	case safeguard.All:
		return true
	case safeguard.GitUntracked:
		return !git.CheckUntracked || r.DisableSafeguards.Has(keyword, safeguard.Git)
	case safeguard.GitUncommitted:
		return !git.CheckUncommitted || r.DisableSafeguards.Has(keyword, safeguard.Git)
	case safeguard.GitOutOfSync:
		return !git.CheckRemote.ValueOr(true) || r.DisableSafeguards.Has(keyword, safeguard.Git)
	case safeguard.Outdated:
		return !run.CheckGenCode || r.DisableSafeguards.Has(keyword)
	default:
		panic(errors.E(errors.ErrInternal, "keyword not supported"))
	}
}

func hclAttrErr(attr *hcl.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrTerramateSchema, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}

func attrErr(attr ast.Attribute, msg string, args ...interface{}) error {
	return errors.E(ErrTerramateSchema, attr.Expr.Range(), fmt.Sprintf(msg, args...))
}
